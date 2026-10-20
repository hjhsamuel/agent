package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session/compaction"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

func (s *Service) getAgent(user string, cfg *AgentConfig) (agent.Agent, *schema.ModelConfig, error) {
	m, llmModel, err := s.randomModel(schema.ChatModel)
	if err != nil {
		return nil, nil, err
	}

	toolPack := s.tool.All(user)

	var subagents []agent.Agent
	if len(cfg.SubAgents) != 0 {
		subagents = append(slices.Clone(toolPack.SubAgents), cfg.SubAgents...)
	} else {
		subagents = toolPack.SubAgents
	}

	agt, err := llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Prompt,
		SubAgents:   subagents,
		Tools:       toolPack.Tools,
		Toolsets:    toolPack.Toolsets,
		Model:       llmModel,
	})
	if err != nil {
		return nil, nil, err
	}

	return agt, &m.Options, nil
}

func (s *Service) getRunner(user string, cfg *AgentConfig) (*runner.Runner, error) {
	agt, opts, err := s.getAgent(user, cfg)
	if err != nil {
		return nil, err
	}

	return runner.New(runner.Config{
		AppName:           schema.MainAgentName,
		Agent:             agt,
		SessionService:    s.adkSession,
		AutoCreateSession: true,
		Compaction: &compaction.Config{
			TokenThreshold:     int(float64(opts.ContextLimit) * CompactRatio),
			EventRetentionSize: 3,
		},
	})
}

func (s *Service) acquireProducer(id string) (context.Context, *ringbuffer.RingBufferProducer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if s.setLife(id, cancel) {
		return nil, nil, errors.New("session already running")
	}

	producer, err := s.rbs.AcquireProducer(ctx, id)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return ctx, producer, nil
}

func (s *Service) Run(user, id string, content string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid session")
	}

	objs, err := s.store.FindSession(bson.M{
		"_id":   sessionId,
		"user":  user,
		"state": bson.M{"$in": []schema.SessionStatus{schema.SessionDone, schema.SessionTemp}},
	})
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return errors.New("session not found")
	}

	ctx, producer, err := s.acquireProducer(sessionId.Hex())
	if err != nil {
		return err
	}

	// 多轮对话、首次对话
	obj, err := s.store.FindOneAndUpdateSession(
		bson.M{
			"_id":   sessionId,
			"state": bson.M{"$in": []schema.SessionStatus{schema.SessionDone, schema.SessionTemp}},
			"user":  user,
		},
		bson.M{"$set": bson.M{
			"state":      schema.SessionRunning,
			"updated_at": time.Now(),
		}},
		options.FindOneAndUpdate().SetProjection(bson.M{"state": 1}),
	)
	if err != nil {
		s.cancel(sessionId.Hex())
		return err
	}

	// 添加消息
	_ = s.store.AddMessage(&schema.Message{
		Session:   sessionId,
		Role:      schema.RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	})

	msg := genai.NewContentFromText(content, genai.RoleUser)
	s.wg.Add(1)
	go s.newTurn(ctx, producer, user, sessionId, msg, obj.State == schema.SessionTemp)

	return nil
}

func (s *Service) newTurn(
	ctx context.Context,
	producer *ringbuffer.RingBufferProducer,
	user string,
	sessionId bson.ObjectID,
	msg *genai.Content,
	isFirst bool,
) {
	defer s.wg.Done()
	defer s.cancel(sessionId.Hex())

	r, err := s.getRunner(
		user,
		&AgentConfig{
			Name:        schema.MainAgentName,
			Description: "main agent",
			Prompt:      "You are an AI agent running inside a multi-user agent platform that supports both normal conversations and long-running tasks.",
		},
	)
	if err != nil {
		_ = s.store.UpdateSession(
			bson.M{"_id": sessionId},
			bson.M{"$set": bson.M{"state": schema.SessionDone}},
		)
		logrus.WithField("Session", sessionId).WithError(err).Error("create agent")
		return
	}

	if isFirst && msg != nil && len(msg.Parts) != 0 && msg.Parts[0].Text != "" {
		name, err := s.ExtractSessionTopic(msg.Parts[0].Text)
		if err != nil {
			name = "新对话"
		}
		_ = s.store.UpdateSession(bson.M{"_id": sessionId}, bson.M{"$set": bson.M{"name": name}})
	}

	message := msg

	for {
		next, err := s.run(ctx, user, sessionId, r, producer, message)
		if err != nil {
			logrus.WithField("Session", sessionId).WithError(err).Error("run agent")
			return
		}
		if !next {
			_ = s.store.UpdateSession(
				bson.M{"_id": sessionId},
				bson.M{"$set": bson.M{"state": schema.SessionDone}},
			)
			return
		}

		// 等待用户输入
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 10):

		}

		obj, err := s.store.FindOneAndUpdateSession(
			bson.M{"_id": sessionId, "state": schema.SessionRunning},
			bson.M{
				"$set": bson.M{"updated_at": time.Now()},
				//"$unset": bson.M{"wait": 1},
			},
		)
		if err != nil {
			return
		}

		parts := make([]*genai.Part, 0, len(obj.Wait))
		for _, item := range obj.Wait {
			parts = append(parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:   item.ID,
					Name: toolconfirmation.FunctionCallName,
					Response: map[string]any{
						"confirmed": true,
						"payload":   item.Content,
					},
				},
			})
		}
		message = buildInputRequiredMessage(obj.Wait)
	}
}

func (s *Service) run(
	ctx context.Context,
	user string,
	sessionId bson.ObjectID,
	r *runner.Runner,
	producer *ringbuffer.RingBufferProducer,
	msg *genai.Content,
) (bool, error) {
	inputRequiredReqs := make([]*schema.WaitInput, 0)

	events := r.Run(ctx, user, sessionId.Hex(), msg, agent.RunConfig{StreamingMode: agent.StreamingModeSSE})
	for event, err := range events {
		if err != nil {
			_ = s.store.UpdateSession(
				bson.M{"_id": sessionId, "state": schema.SessionRunning},
				bson.M{"$set": bson.M{"state": schema.SessionDone}},
			)
			return false, err
		}

		if event == nil {
			continue
		}

		// stream
		if event.Partial {
			if event.Content == nil {
				continue
			}
			if event.Author == schema.MainAgentName {
				for _, part := range event.Content.Parts {
					if part == nil || part.Text == "" {
						continue
					}

					chunk := ringbuffer.SSEChunk{Data: part.Text}
					if part.Thought {
						chunk.Type = ringbuffer.SSEChunkTypeReasoning
					} else {
						chunk.Type = ringbuffer.SSEChunkTypeData
					}
					_, _ = s.rbs.Push(producer, chunk)
				}
			}
			continue
		}

		if event.Content == nil {
			continue
		}

		var content strings.Builder
		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}

			// 只将来自主 agent 的模型信息进行保存
			if !part.Thought && event.Content.Role == "model" && part.Text != "" && event.Author == schema.MainAgentName {
				content.WriteString(part.Text)
				_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeEnd})
			}

			// 处理 input_required
			if part.FunctionCall != nil && part.FunctionCall.Name == toolconfirmation.FunctionCallName {
				value, ok := part.FunctionCall.Args["toolConfirmation"]
				if !ok || value == nil {
					continue
				}
				data, err := json.Marshal(value)
				if err != nil {
					continue
				}
				var confirmation toolconfirmation.ToolConfirmation
				if err = json.Unmarshal(data, &confirmation); err != nil {
					continue
				}
				inputRequiredReqs = append(inputRequiredReqs, &schema.WaitInput{
					ID:   part.FunctionCall.ID,
					Hint: confirmation.Hint,
				})
			}
		}
		// 添加展示信息
		if content.Len() != 0 {
			_ = s.store.AddMessage(&schema.Message{
				Session:   sessionId,
				Role:      schema.RoleAssistant,
				Content:   content.String(),
				CreatedAt: time.Now(),
			})
		}
	}

	if len(inputRequiredReqs) != 0 {
		_ = s.store.UpdateSession(
			bson.M{"_id": sessionId, "state": schema.SessionRunning},
			bson.M{"$set": bson.M{"state": schema.SessionWait, "wait": inputRequiredReqs}},
		)
		for _, item := range inputRequiredReqs {
			_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
				Type: ringbuffer.SSEChunkTypeHumanInTheLoop,
				Data: ringbuffer.HITLEvent{ID: item.ID, Content: item.Content}.ToData(),
			})
		}

		return true, nil
	}

	return false, nil
}

func (s *Service) SetInputRequiredInfo(user, id string, toolCallId string, content string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	obj, err := s.store.FindOneAndUpdateSession(
		bson.M{
			"_id":   sessionId,
			"state": schema.SessionWait,
			"wait":  bson.M{"$elemMatch": bson.M{"id": toolCallId, "content": ""}},
		},
		bson.M{"$set": bson.M{"wait.$.content": content}},
	)
	if err != nil {
		return err
	}

	// 检查是否补充完毕
	parts := make([]*genai.Part, 0, len(obj.Wait))
	for _, item := range obj.Wait {
		if item.ID == toolCallId {
			parts = append(parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					ID:   item.ID,
					Name: toolconfirmation.FunctionCallName,
					Response: map[string]any{
						"confirmed": true,
						"payload":   content,
					},
				},
			})
			continue
		}

		if item.Content == "" {
			return nil
		}

		parts = append(parts, &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				ID:   item.ID,
				Name: toolconfirmation.FunctionCallName,
				Response: map[string]any{
					"confirmed": true,
					"payload":   item.Content,
				},
			},
		})
	}
	err = s.store.UpdateSession(
		bson.M{"_id": sessionId, "state": schema.SessionWait},
		bson.M{"$set": bson.M{"state": schema.SessionRunning}},
	)
	if err != nil {
		return nil
	}

	for i := 0; i < 3; i++ {
		if !s.lifeExists(id) {
			// 再次校验会话状态
			objs, err := s.store.FindSession(bson.M{"_id": sessionId, "state": schema.SessionRunning})
			if err != nil {
				return err
			}
			if len(objs) == 0 {
				return nil
			}

			ctx, producer, err := s.acquireProducer(id)
			if err != nil {
				return err
			}

			msg := &genai.Content{Role: genai.RoleUser, Parts: parts}
			s.wg.Add(1)
			go s.newTurn(ctx, producer, user, sessionId, msg, false)
			return nil
		}
		time.Sleep(time.Second * 3)
	}
	// 原协程仍在运行，直接退出
	return nil
}
