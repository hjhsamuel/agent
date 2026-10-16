package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
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
		SessionService:    adapter.NewSessionService(s.store),
		AutoCreateSession: true,
		Compaction: &compaction.Config{
			TokenThreshold:     int(float64(opts.ContextLimit) * CompactRatio),
			EventRetentionSize: 3,
		},
	})
}

func (s *Service) Run(user, id string, content string) error {
	var sessionId bson.ObjectID
	var err error
	if id != "" {
		// 多轮对话
		sessionId, err = bson.ObjectIDFromHex(id)
		if err != nil {
			return errors.New("invalid session")
		}
		err = s.store.UpdateSession(
			bson.M{"_id": sessionId, "state": schema.SessionDone, "user": user},
			bson.M{"$set": bson.M{"state": schema.SessionRunning, "updated_at": time.Now()}},
		)
		if err != nil {
			return err
		}
	} else {
		// 创建会话
		sessionId, err = s.store.CreateTempSession(&schema.Session{
			User:      user,
			Name:      content[:20] + "...", // 暂时粗暴的截取前20个字符，后续做主题提取
			State:     schema.SessionTemp,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
		if err != nil {
			return err
		}
		id = sessionId.Hex()
	}

	msg := genai.NewContentFromText(content, genai.RoleUser)
	return s.newTurn(user, sessionId, msg)
}

func (s *Service) newTurn(
	user string,
	sessionId bson.ObjectID,
	msg *genai.Content,
) error {
	r, err := s.getRunner(
		user,
		&AgentConfig{
			Name:        schema.MainAgentName,
			Description: "main agent",
			Prompt:      "You are an AI agent running inside a multi-user agent platform that supports both normal conversations and long-running tasks.",
		},
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	producer, err := s.rbs.AcquireProducer(ctx, sessionId.Hex())
	if err != nil {
		cancel()
		return err
	}
	s.setLife(sessionId.Hex(), cancel)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.cancel(sessionId.Hex())

		message := msg

		for {
			next, err := s.run(ctx, user, sessionId, r, producer, message)
			if err != nil {
				return
			}
			if !next {
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
	}()

	return nil
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

	for i := 0; i < 2; i++ {
		if !s.lifeExists(id) {
			msg := &genai.Content{Role: genai.RoleUser, Parts: parts}
			return s.newTurn(user, sessionId, msg)
		}
		time.Sleep(time.Second * 3)
	}
	// 原协程仍在运行，直接退出
	return nil
}
