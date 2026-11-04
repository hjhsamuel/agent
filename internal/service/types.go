package service

import (
	"context"
	"iter"
	"sync"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/server/adka2a/v2"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

const CompactRatio = 0.6

type sessionLife struct {
	context.Context
	cancel     context.CancelFunc
	finishOnce sync.Once
}

func buildInputRequiredMessage(inputs []*schema.WaitInput) *genai.Content {
	parts := make([]*genai.Part, 0, len(inputs))
	for _, item := range inputs {
		// Preserve the correlation ID through the runner. The A2A callback
		// converts text-only answers at dispatch after restoring TaskID.
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
	return &genai.Content{Role: "user", Parts: parts}
}

func reflectModelLevel(level schema.AgentLevel) []schema.ModelLevel {
	switch level {
	case schema.AgentLevelLow:
		return []schema.ModelLevel{
			schema.ModelLevelLow,
		}
	case schema.AgentLevelMedium:
		return []schema.ModelLevel{
			schema.ModelLevelMedium,
			schema.ModelLevelHigh,
		}
	case schema.AgentLevelHigh:
		return []schema.ModelLevel{
			schema.ModelLevelExtraHigh,
			schema.ModelLevelUltra,
		}
	default:
		return []schema.ModelLevel{}
	}
}

type AgentGroup struct {
	ID          bson.ObjectID
	Name        string
	Description string
	Prompt      string
	Level       schema.AgentLevel
	Agents      []*AgentGroup
}

type Runner struct {
	Name string
	*runner.Runner
}

// RunWithRemoteResults lets the main model summarize a completed remote task.
// A nil message continues from persisted history without adding another user turn.
func (r *Runner) RunWithRemoteResults(ctx context.Context, user, id string, msg *genai.Content, cfg agent.RunConfig) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		for {
			var last *session.Event
			for event, err := range r.Run(ctx, user, id, msg, cfg) {
				if !yield(event, err) || err != nil {
					return
				}
				if event != nil && !event.Partial && event.Actions.Compaction == nil {
					last = event
				}
			}
			if ctx.Err() != nil || last == nil || last.Author == r.Name {
				return
			}
			response, _ := last.CustomMetadata[adka2a.ToADKMetaKey("response")].(map[string]any)
			status, _ := response["status"].(map[string]any)
			if status["state"] != string(a2a.TaskStateCompleted) {
				return
			}
			msg = nil
		}
	}
}
