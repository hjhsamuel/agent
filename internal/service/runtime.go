package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hjhsamuel/agent/internal/service/prompts"
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

	toolPack := s.tool.All(user, cfg.Tag...)
	agt, err := llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Prompt,
		SubAgents:   toolPack.SubAgents,
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
		AppName:           schema.RunnerName,
		Agent:             agt,
		SessionService:    s.adkSession,
		AutoCreateSession: true,
		Compaction: &compaction.Config{
			TokenThreshold:     int(float64(opts.ContextLimit) * CompactRatio),
			EventRetentionSize: 3,
		},
	})
}

func (s *Service) acquireProducer(id string) (*sessionLife, *ringbuffer.RingBufferProducer, error) {
	s.lifecycleMu.Lock()
	if s.stopping.Load() {
		s.lifecycleMu.Unlock()
		return nil, nil, errors.New("service is shutting down")
	}
	ctx, cancel := context.WithCancel(context.Background())
	life := &sessionLife{Context: ctx, cancel: cancel}
	if s.setLife(id, life) {
		s.lifecycleMu.Unlock()
		cancel()
		return nil, nil, errors.New("session already running")
	}
	// Register before Close can start waiting, including synchronous setup work.
	s.wg.Add(1)
	s.lifecycleMu.Unlock()

	producer, err := s.rbs.AcquireProducer(ctx, id)
	if err != nil {
		s.finishLife(id, life)
		return nil, nil, err
	}

	return life, producer, nil
}

func (s *Service) Run(user, id string, content string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid session")
	}

	r, err := s.getRunner(
		user,
		&AgentConfig{
			Name:        schema.MainAgentName,
			Description: "main agent",
			Prompt:      prompts.SystemPrompt,
		},
	)
	if err != nil {
		return err
	}

	ctx, producer, err := s.acquireProducer(sessionId.Hex())
	if err != nil {
		return err
	}

	// 多轮对话、首次对话
	state, turnId, err := s.store.AddMessage(&schema.Message{
		Session:   sessionId,
		Turn:      0,
		Seq:       0,
		Role:      schema.RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	})
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}

	msg := genai.NewContentFromText(content, genai.RoleUser)
	go s.newTurn(ctx, producer, user, sessionId, turnId, r, msg, state == schema.SessionTemp)

	return nil
}

func (s *Service) newTurn(
	ctx *sessionLife,
	producer *ringbuffer.RingBufferProducer,
	user string,
	sessionId bson.ObjectID,
	turnId int64,
	r *runner.Runner,
	msg *genai.Content,
	isFirst bool,
) {
	defer s.finishLife(sessionId.Hex(), ctx)
	if ctx.Err() != nil {
		return
	}

	if isFirst {
		if msg != nil && len(msg.Parts) != 0 && msg.Parts[0].Text != "" {
			name, err := s.ExtractSessionTopic(ctx, msg.Parts[0].Text)
			if err != nil {
				name = "新对话"
			}
			if ctx.Err() != nil {
				return
			}
			_ = s.store.UpdateSession(bson.M{"_id": sessionId}, bson.M{"$set": bson.M{"name": name}})
		}
	}

	waitInputs, err := s.run(ctx, user, sessionId, turnId, r, producer, msg)
	// Shutdown is an interruption, not a completed/failed turn. Preserve the
	// persisted running state so Start can resume it, even if the runner ended
	// without returning a cancellation error.
	if s.stopping.Load() && ctx.Err() != nil {
		return
	}
	if err != nil {
		goto Error
	}
	goto Success

Error:
	_, _, _ = s.store.AddMessage(&schema.Message{
		Session:   sessionId,
		Turn:      turnId,
		Role:      schema.RoleError,
		Content:   err.Error(),
		CreatedAt: time.Now(),
	})
	_ = s.store.UpdateSession(
		bson.M{"_id": sessionId, "state": schema.SessionRunning},
		bson.M{"$set": bson.M{"state": schema.SessionDone}},
	)

	s.releaseLife(sessionId.Hex(), ctx)
	_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
		Type: ringbuffer.SSEChunkTypeError,
		Data: err.Error(),
	})
	_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
		Type: ringbuffer.SSEChunkTypeEnd,
	})

	return

Success:
	if len(waitInputs) != 0 {
		err = s.store.UpdateSession(
			bson.M{"_id": sessionId, "state": schema.SessionRunning},
			bson.M{"$set": bson.M{"state": schema.SessionWait, "wait": waitInputs}},
		)
		if err != nil {
			goto Error
		}

		s.releaseLife(sessionId.Hex(), ctx)
		for _, item := range waitInputs {
			_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{
				Type: ringbuffer.SSEChunkTypeHumanInTheLoop,
				Data: ringbuffer.HITLEvent{ID: item.ID, Content: item.Hint}.ToData(),
			})
		}
	} else {
		err = s.store.UpdateSession(
			bson.M{"_id": sessionId},
			bson.M{"$set": bson.M{"state": schema.SessionDone, "wait": []*schema.WaitInput{}}},
		)
		if err != nil {
			goto Error
		}
		s.releaseLife(sessionId.Hex(), ctx)
		_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeEnd})
	}
	return
}

func (s *Service) run(
	ctx context.Context,
	user string,
	sessionId bson.ObjectID,
	turnId int64,
	r *runner.Runner,
	producer *ringbuffer.RingBufferProducer,
	msg *genai.Content,
) ([]*schema.WaitInput, error) {
	inputRequiredReqs := make([]*schema.WaitInput, 0)

	events := r.Run(ctx, user, sessionId.Hex(), msg, agent.RunConfig{StreamingMode: agent.StreamingModeSSE})
	for event, err := range events {
		if err != nil {
			return nil, err
		}

		if event == nil {
			continue
		}

		// stream
		if event.Partial {
			if event.Content == nil {
				continue
			}
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
					ID:        part.FunctionCall.ID,
					Hint:      confirmation.Hint,
					CreatedAt: time.Now(),
				})
			}
		}
		// 添加展示信息
		if content.Len() != 0 {
			_, _, err := s.store.AddMessage(&schema.Message{
				Session:   sessionId,
				Turn:      turnId,
				Role:      schema.RoleAssistant,
				Content:   content.String(),
				CreatedAt: time.Now(),
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return inputRequiredReqs, nil
}

func (s *Service) SetInputRequiredInfo(user, id string, toolCallId string, content string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	obj, err := s.store.FindOneAndUpdateSession(
		bson.M{
			"_id":   sessionId,
			"user":  user,
			"state": schema.SessionWait,
			"wait":  bson.M{"$elemMatch": bson.M{"id": toolCallId, "content": ""}},
		},
		bson.M{"$set": bson.M{"wait.$.content": content, "wait.$.updated_at": time.Now()}},
	)
	if err != nil {
		return err
	}

	turnId := obj.TurnSeq - 1

	// 检查是否补充完毕
	var (
		parts    = make([]*genai.Part, 0, len(obj.Wait))
		messages = make([]*schema.Message, 0)
	)
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
			messages = append(messages, &schema.Message{
				Session:   sessionId,
				Turn:      turnId,
				Role:      schema.RoleUser,
				Content:   fmt.Sprintf("> %s \n\n%s", item.Hint, content),
				CreatedAt: item.UpdatedAt,
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
		messages = append(messages, &schema.Message{
			Session:   sessionId,
			Turn:      turnId,
			Role:      schema.RoleUser,
			Content:   fmt.Sprintf("> %s \n\n%s", item.Hint, item.Content),
			CreatedAt: item.UpdatedAt,
		})
	}

	ctx, producer, err := s.acquireProducer(sessionId.Hex())
	if err != nil {
		return err
	}

	r, err := s.getRunner(
		user,
		&AgentConfig{
			Name:        schema.MainAgentName,
			Description: "main agent",
			Prompt:      prompts.SystemPrompt,
		},
	)
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}

	err = s.store.FinishWaitInput(sessionId, messages...)
	if err != nil {
		s.finishLife(sessionId.Hex(), ctx)
		return err
	}

	msg := &genai.Content{Role: genai.RoleUser, Parts: parts}
	go s.newTurn(ctx, producer, user, sessionId, turnId, r, msg, false)

	return nil
}
