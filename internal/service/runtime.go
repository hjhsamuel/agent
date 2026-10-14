package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
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
		AppName:           schema.MainAgentName,
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
		Name:        schema.MainAgentName,
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

			chunks, err := runtimeEventChunks(event)
			if err != nil {
				return
			}
			for _, chunk := range chunks {
				if _, err := s.rbs.Push(producer, chunk); err != nil {
					return
				}
			}
		}
	}()

	return nil
}

func runtimeEventChunks(event *session.Event) ([]ringbuffer.SSEChunk, error) {
	if event == nil {
		return nil, nil
	}
	// ADK uses LongRunningToolIDs for both remote A2A input_required and
	// tool input/confirmation requests. Wait for complete arguments before
	// publishing the request ID and original input details.
	if !event.Partial && (len(event.LongRunningToolIDs) > 0 || event.RequestedInput != nil) {
		return humanInTheLoopChunks(event)
	}
	if event.Content == nil {
		return nil, nil
	}

	var chunks []ringbuffer.SSEChunk
	for _, part := range event.Content.Parts {
		if part == nil || part.Text == "" {
			continue
		}
		if !event.Partial {
			// 这是一条完整的消息，传递结束标识。
			chunks = append(chunks, ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeEnd})
			continue
		}
		// 普通流式文本只处理主 agent；交互请求不受此限制。
		if event.Author != schema.MainAgentName {
			continue
		}
		chunk := ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeData, Data: []byte(part.Text)}
		if part.Thought {
			chunk.Type = ringbuffer.SSEChunkTypeReasoning
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func humanInTheLoopChunks(event *session.Event) ([]ringbuffer.SSEChunk, error) {
	var requests []*HumanInTheLoopRequest
	byID := make(map[string]*HumanInTheLoopRequest)
	requestFor := func(id string) *HumanInTheLoopRequest {
		if request := byID[id]; request != nil {
			return request
		}
		request := &HumanInTheLoopRequest{ID: id}
		byID[id] = request
		requests = append(requests, request)
		return request
	}
	for _, id := range event.LongRunningToolIDs {
		requestFor(id)
	}
	if input := event.RequestedInput; input != nil {
		request := requestFor(input.InterruptID)
		request.Message = input.Message
		request.Payload = input.Payload
		request.ResponseSchema = input.ResponseSchema
	}

	var messages []string
	if event.Content != nil {
		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}
			if part.Text != "" && !part.Thought {
				messages = append(messages, part.Text)
			}
			if call := part.FunctionCall; call != nil {
				if request := byID[call.ID]; request != nil {
					request.Name, request.Args = call.Name, call.Args
				}
			}
			if response := part.FunctionResponse; response != nil {
				if request := byID[response.ID]; request != nil {
					if request.Name == "" {
						request.Name = response.Name
					}
					request.Response = response.Response
				}
			}
		}
	}
	chunks := make([]ringbuffer.SSEChunk, 0, len(requests))
	for _, request := range requests {
		if request.Message == "" {
			request.Message = strings.Join(messages, "\n")
		}
		data, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, ringbuffer.SSEChunk{Type: ringbuffer.SSEChunkTypeHumanInTheLoop, Data: data})
	}
	return chunks, nil
}
