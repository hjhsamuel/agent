package service

import (
	"context"
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session/compaction"
	"google.golang.org/genai"
)

func (s *Service) getRunner(cfg *AgentConfig) (*runner.Runner, error) {
	model, llmModel, err := s.randomModel(schema.ChatModel)
	if err != nil {
		return nil, err
	}
	agt, err := llmagent.New(llmagent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Instruction: cfg.Prompt,
		SubAgents:   s.tool.SubAgents,
		Tools:       s.tool.Tools,
		Toolsets:    s.tool.Toolsets,
		Model:       llmModel,
	})

	return runner.New(runner.Config{
		AppName:           "root",
		Agent:             agt,
		SessionService:    adapter.NewSessionService(s.store),
		AutoCreateSession: true,
		Compaction: &compaction.Config{
			TokenThreshold:     int(float64(model.Options.ContextLimit) * CompactRatio),
			EventRetentionSize: 3,
		},
	})
}

func (s *Service) Run(user, id string, content string) error {
	if id != "" {
		sessionId, err := bson.ObjectIDFromHex(id)
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
		sessionId, err := s.store.CreateTempSession(&schema.Session{
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

	r, err := s.getRunner(&AgentConfig{
		Name:        "root",
		Description: "main agent",
		Prompt:      "You are an AI agent running inside a multi-user agent platform that supports both normal conversations and long-running tasks.",
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	producer, err := s.rbs.AcquireProducer(ctx, id)
	if err != nil {
		cancel()
		return err
	}
	s.setLife(id, cancel)

	go func() {
		defer s.cancel(id)

		events := r.Run(
			ctx,
			user,
			id,
			genai.NewContentFromText(content, genai.RoleUser),
			agent.RunConfig{
				StreamingMode: agent.StreamingModeSSE,
			},
		)
		for event, err := range events {
			if err != nil {
				return
			}
			if event == nil || event.Content == nil || event.Author != "root" {
				continue
			}

			if !event.Partial {
				for _, part := range event.Content.Parts {
					if part.Text != "" {
						_, _ = s.rbs.Push(producer, ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeEnd})
					}
				}
			} else {
				for _, part := range event.Content.Parts {
					if part == nil || part.Text == "" {
						continue
					}

					chunk := ringbuffer.SSEChunk{
						Data: []byte(part.Text),
					}
					if part.Thought {
						chunk.Type = ringbuffer.SSEChunkTypeReasoning
					} else {
						chunk.Type = ringbuffer.SSEChunkTypeData
					}
					_, _ = s.rbs.Push(producer, chunk)
				}
			}
		}
	}()

	return nil
}
