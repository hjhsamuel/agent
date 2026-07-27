package provider

import (
	"context"
	"encoding/json"

	"github.com/hjhsamuel/agent/internal/tools"
)

type OnChunk func(content string)

type Provider interface {
	Chat(ctx context.Context, req *ChatReq, onChunk OnChunk) (*ChatRsp, error)
	Embedding(ctx context.Context, model string, messages []string) ([][]float64, error)
}

type ChatReq struct {
	Model        string
	SystemPrompt string
	Messages     []*Message
	Tools        []*tools.ToolDefine
	MaxTokens    int64
}

type MessageRole int

const (
	SystemMessage MessageRole = iota
	UserMessage
	AssistantMessage
	ToolMessage
)

type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []*ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
	IsError    bool        `json:"is_error,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ChatRsp struct {
	Message      *Message
	FinishReason string
}
