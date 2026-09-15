package tool

import (
	"context"

	"github.com/openai/openai-go/v3"
)

const ()

type Tool interface {
	Type() ToolType
	Define() openai.ChatCompletionToolUnionParam
	Execute(ctx context.Context, token, content string) (*ToolResult, error)
	Resume(ctx context.Context, token, contextId, taskId string, content string) (*ToolResult, error)
	Check(ctx context.Context, token, contextId, taskId string) (*ToolResult, error)
}

type ToolType string

const (
	LocalTool    ToolType = "local"
	SubAgentTool ToolType = "subagent"
	McpTool      ToolType = "mcp"
	A2ATool      ToolType = "a2a"
)

type ToolResult struct {
	ContextId string
	TaskId    string
	Status    TaskStatus
	Content   string
}

type TaskStatus string

const (
	TaskSubmitted     TaskStatus = "submitted"
	TaskWorking       TaskStatus = "working"
	TaskInputRequired TaskStatus = "input_required"
	TaskCompleted     TaskStatus = "completed"
	TaskFailed        TaskStatus = "failed"
	TaskCanceled      TaskStatus = "canceled"
	TaskRejected      TaskStatus = "rejected"
	TaskAuthRequired  TaskStatus = "auth_required"
)
