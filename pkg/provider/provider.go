package provider

import (
	"context"

	"github.com/openai/openai-go/v3"
)

type Provider interface {
	Chat(ctx context.Context, model string, prompt string, messages []*Message, conf *ChatConfig) (*Message, error)
	Stream(ctx context.Context, model string, prompt string, messages []*Message, conf *ChatConfig, yield YieldFunc) (*Message, error)
}

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`

	ToolCalls  []*ToolCall `json:"tool_calls"`
	ToolCallId string      `json:"tool_call_id"`

	Reasoning string      `json:"reasoning"`
	Usage     *TokenUsage `json:"usage"`
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type TokenUsage struct {
	Total       int64
	Prompt      int64
	Cached      int64
	Completions int64
	Reasoning   int64
}

type ChatConfig struct {
	Temperature    float64
	Tool           []openai.ChatCompletionToolUnionParam
	ResponseFormat openai.ChatCompletionNewParamsResponseFormatUnion
	Extra          map[string]any
}

type YieldFunc func(chunk *StreamChunk, err error) error

type StreamChunk struct {
	Type    ContentType
	Content string
}

type ContentType int

const (
	Reasoning ContentType = iota
	Completion
)
