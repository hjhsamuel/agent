package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const subAgentToolName = "subagent"

type subAgentCallKey struct{}
type subAgentCall struct {
	message bson.ObjectID
	id      string
}

// Each tool instance is bound to exactly one main agent, never the global tool manager.
type subAgentTool struct{ owner *Agent }

var _ tool.Tool = (*subAgentTool)(nil)

func (t *subAgentTool) Type() tool.ToolType { return tool.SubAgentTool }

func (t *subAgentTool) Define() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{OfFunction: &openai.ChatCompletionFunctionToolParam{
		Type: "function", Function: shared.FunctionDefinitionParam{
			Name:        subAgentToolName,
			Description: openai.String("Delegate a self-contained task to a subagent with independent conversation history and the same tools. Supply all necessary context in task. The result is returned when the task finishes. Subagents cannot delegate further. For input_required, provide inputs containing the requested context_id, task_id and answer in content."),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "Complete task instructions and context (required for a new task).",
					},
					"inputs": map[string]any{
						"type":        "array",
						"description": "Only used when resuming input_required tasks.",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"context_id": map[string]any{
									"type": "string",
								},
								"task_id": map[string]any{
									"type": "string",
								},
								"content": map[string]any{
									"type": "string",
								},
							},
							"required":             []string{"context_id", "task_id", "content"},
							"additionalProperties": false,
						},
					},
				}, "additionalProperties": false,
			},
		},
	}}
}

func (t *subAgentTool) Execute(ctx context.Context, token, content string) (*tool.ToolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var params struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(content), &params); err != nil {
		return &tool.ToolResult{Status: tool.TaskFailed, Content: "invalid arguments: " + err.Error()}, nil
	}
	if strings.TrimSpace(params.Task) == "" {
		return &tool.ToolResult{Status: tool.TaskFailed, Content: "task is required"}, nil
	}
	call, ok := ctx.Value(subAgentCallKey{}).(subAgentCall)
	if !ok || call.id == "" {
		return nil, errors.New("subagent requires an owning tool call")
	}
	m := t.owner.runtime.main
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || t.owner.ctx.Err() != nil {
		return nil, errors.New("main agent is stopped")
	}
	id, err := t.owner.base.Store.CreateSubAgentTask(t.owner.id, call.message, call.id, params.Task)
	if err != nil {
		return nil, err
	}
	if _, err := t.owner.childLocked(id); err != nil {
		if t.owner.ctx.Err() != nil {
			return nil, errors.Join(err, t.owner.finishChildTask(t.owner.id, id, tool.TaskCanceled, t.owner.ctx.Err().Error()))
		}
		return nil, err
	}
	return &tool.ToolResult{ContextId: id.Hex(), TaskId: id.Hex(), Status: tool.TaskSubmitted}, nil
}

func (t *subAgentTool) lookup(ctx context.Context, contextID, taskID string) (*Agent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contextID != taskID {
		return nil, errors.New("invalid subagent context")
	}
	id, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return nil, err
	}
	return t.owner.child(id)
}

func (t *subAgentTool) Check(ctx context.Context, token, contextID, taskID string) (*tool.ToolResult, error) {
	child, err := t.lookup(ctx, contextID, taskID)
	if err != nil {
		return nil, err
	}
	state, err := child.GetState()
	if err != nil {
		return nil, err
	}
	return &tool.ToolResult{ContextId: contextID, TaskId: taskID, Status: state.Status, Content: strings.Join(state.Artifacts, "\n")}, nil
}

func (t *subAgentTool) Resume(ctx context.Context, token, contextID, taskID, content string) (*tool.ToolResult, error) {
	child, err := t.lookup(ctx, contextID, taskID)
	if err != nil {
		return nil, err
	}
	state, err := child.GetState()
	if err != nil {
		return nil, err
	}
	// A previous resume may have committed before its acknowledgement was
	// lost. Return the current state rather than replaying the nested input.
	if state.Status != tool.TaskInputRequired {
		return t.Check(ctx, token, contextID, taskID)
	}
	if err := child.Resume(content); err != nil {
		return nil, err
	}
	return t.Check(ctx, token, contextID, taskID)
}
