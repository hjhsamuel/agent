package service

import (
	"context"
	"sync"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
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

func isA2AInputRequired(event *session.Event) bool {
	response, _ := event.CustomMetadata[adka2a.ToADKMetaKey("response")].(map[string]any)
	status, _ := response["status"].(map[string]any)
	state, _ := status["state"].(string)
	return state == string(a2a.TaskStateInputRequired)
}

func buildInputRequiredMessage(inputs []*schema.WaitInput) *genai.Content {
	parts := make([]*genai.Part, 0, len(inputs))
	for _, item := range inputs {
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
