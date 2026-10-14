package service

import (
	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

// HumanInTheLoopRequest is the SSE payload for one pending input request.
// ID is the tool call or interrupt ID used to correlate the user's reply.
type HumanInTheLoopRequest struct {
	ID             string             `json:"id"`
	Name           string             `json:"name,omitempty"`
	Message        string             `json:"message,omitempty"`
	Args           map[string]any     `json:"args,omitempty"`
	Response       map[string]any     `json:"response,omitempty"`
	Payload        any                `json:"payload,omitempty"`
	ResponseSchema *jsonschema.Schema `json:"responseSchema,omitempty"`
}

const CompactRatio = 0.6

type AgentConfig struct {
	Name        string
	Description string
	Prompt      string
}

type Tool struct {
	SubAgents []agent.Agent
	Tools     []tool.Tool
	Toolsets  []tool.Toolset
}
