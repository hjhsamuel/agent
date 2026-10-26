package service

import (
	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/server/adka2a/v2"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

const CompactRatio = 0.6

type AgentConfig struct {
	Name        string
	Description string
	Prompt      string
	Tag         []string
}

type Tool struct {
	SubAgents []agent.Agent
	Tools     []tool.Tool
	Toolsets  []tool.Toolset
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
