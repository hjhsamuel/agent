package a2a

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

type A2A struct {
	id          string
	description string

	client *a2aclient.Client
}

func (a *A2A) Type() tool.ToolType {
	return tool.A2ATool
}

func (a *A2A) Define() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Type: "function",
			Function: shared.FunctionDefinitionParam{
				Name:        a.id,
				Description: openai.String(a.description),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{
							"type":        "string",
							"description": "The content to send to the remote agent",
						},
					},
					"required":             []string{"content"},
					"additionalProperties": false,
				},
			},
		},
	}
}

func (a *A2A) buildContext(ctx context.Context, token string) context.Context {
	return a2aclient.AttachServiceParams(ctx, a2aclient.ServiceParams{"Authorization": {"Bearer " + token}})
}

func (a *A2A) Execute(
	ctx context.Context,
	token string,
	content string,
) (*tool.ToolResult, error) {
	message := a2a.NewMessage(a2a.MessageRoleUser, a2a.NewTextPart(content))

	result, err := a.client.SendMessage(
		a.buildContext(ctx, token),
		&a2a.SendMessageRequest{
			Message: message,
			Config: &a2a.SendMessageConfig{
				ReturnImmediately: true,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	response := &tool.ToolResult{}
	switch v := result.(type) {
	case *a2a.Task:
		// 任务
		// 由于配置了 return_immediately: true，因此返回的状态应该是 submitted 和 working
		response.ContextId = v.ContextID
		response.TaskId = string(v.ID)
		response.Status = StatusReflect(v.Status.State)
	case *a2a.Message:
		// 简单交互
		response.Content = FormatMessage(v)
	}

	return response, nil
}

func (a *A2A) Resume(
	ctx context.Context,
	token string,
	contextId string,
	taskId string,
	content string,
) (*tool.ToolResult, error) {
	info := a2a.TaskInfo{
		TaskID:    a2a.TaskID(taskId),
		ContextID: contextId,
	}
	message := a2a.NewMessageForTask(a2a.MessageRoleUser, info, a2a.NewTextPart(content))

	result, err := a.client.SendMessage(
		a.buildContext(ctx, token),
		&a2a.SendMessageRequest{
			Message: message,
			Config: &a2a.SendMessageConfig{
				ReturnImmediately: true,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	response := &tool.ToolResult{}
	switch v := result.(type) {
	case *a2a.Task:
		// 任务
		// 由于配置了 return_immediately: true，因此返回的状态应该是 submitted 和 working
		response.ContextId = v.ContextID
		response.TaskId = string(v.ID)
		response.Status = StatusReflect(v.Status.State)
	case *a2a.Message:
		// 简单交互
		response.Content = FormatMessage(v)
	}

	return response, nil
}

func (a *A2A) Check(
	ctx context.Context,
	token string,
	contextId string,
	taskId string,
) (*tool.ToolResult, error) {
	task, err := a.client.GetTask(
		a.buildContext(ctx, token),
		&a2a.GetTaskRequest{
			ID: a2a.TaskID(taskId),
		},
	)
	if err != nil {
		return nil, err
	}

	response := &tool.ToolResult{
		ContextId: contextId,
		TaskId:    taskId,
		Status:    StatusReflect(task.Status.State),
	}
	switch task.Status.State {
	case a2a.TaskStateCompleted:
		response.Content = FormatArtifacts(task.Artifacts)
	case a2a.TaskStateSubmitted, a2a.TaskStateWorking:
	default:
		if task.Status.Message == nil {
			response.Content = "unknown error, remote agent has not providing more details"
		} else {
			response.Content = FormatMessage(task.Status.Message)
		}
	}

	return response, nil
}

func NewA2A(
	id string,
	description string,
	client *a2aclient.Client,
) tool.Tool {
	return &A2A{
		id:          id,
		description: description,
		client:      client,
	}
}

func FormatArtifacts(artifacts []*a2a.Artifact) string {
	result := make([]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		parts := make([]any, 0)
		for _, part := range artifact.Parts {
			switch part.Content.(type) {
			case a2a.Text:
				parts = append(parts, map[string]any{"type": "text", "text": part.Text()})
			case a2a.Raw:
				parts = append(parts, map[string]any{"type": "raw", "raw": part.Raw()})
			case a2a.Data:
				parts = append(parts, map[string]any{"type": "data", "data": part.Data()})
			case a2a.URL:
				parts = append(parts, map[string]any{"type": "url", "url": part.URL()})
			}
		}
		result = append(result, parts)
	}

	content, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return string(content)
}

func FormatMessage(info *a2a.Message) string {
	if info == nil {
		return ""
	}

	var b strings.Builder
	for _, part := range info.Parts {
		switch v := part.Content.(type) {
		case a2a.Text:
			b.WriteString(string(v))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func StatusReflect(status a2a.TaskState) tool.TaskStatus {
	switch status {
	case a2a.TaskStateAuthRequired:
		return tool.TaskAuthRequired
	case a2a.TaskStateCanceled:
		return tool.TaskCanceled
	case a2a.TaskStateCompleted:
		return tool.TaskCompleted
	case a2a.TaskStateFailed:
		return tool.TaskFailed
	case a2a.TaskStateInputRequired:
		return tool.TaskInputRequired
	case a2a.TaskStateRejected:
		return tool.TaskRejected
	case a2a.TaskStateSubmitted:
		return tool.TaskSubmitted
	case a2a.TaskStateWorking:
		return tool.TaskWorking
	default:
		return tool.TaskFailed
	}
}
