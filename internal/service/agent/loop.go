package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/resolver"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/openai/openai-go/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (a *Agent) loopDefer(err error) {
	if a.runtime.main != nil {
		err = errors.Join(err, a.closeChildren())
	}
	select {
	case a.base.Up <- &notify.UpperEvent{ID: a.id.Hex(), Finished: true, Error: err}:
	case <-a.base.Shutdown:
	}
}

func (a *Agent) calculateContextSize(message *provider.Message) int64 {
	if message == nil || message.Usage == nil {
		return 0
	}
	return message.Usage.Prompt + message.Usage.Completions - message.Usage.Reasoning
}

func (a *Agent) loop() {
	defer a.runtime.wg.Done()
	a.loopDefer(a.runLoop())
}

func (a *Agent) emit(event notify.SSEvent) error {
	// Subagent output is collected as a tool result, never a service event.
	if !a.parent.IsZero() {
		return a.ctx.Err()
	}
	select {
	case a.base.Up <- &notify.UpperEvent{ID: a.id.Hex(), Event: event}:
		return nil
	case <-a.ctx.Done():
		return a.ctx.Err()
	}
}

func (a *Agent) mergeMessages() {
	a.runtime.OldMessages = append(a.runtime.OldMessages, a.runtime.NewMessages...)
	a.runtime.NewMessages = nil
	if !a.runtime.latestId.IsZero() {
		a.runtime.oldId = a.runtime.latestId
	}
}

// Estimate bytes conservatively when a provider omits token usage. Include tool
// arguments/results and newly appended user input, not just the last response.
func (a *Agent) contextSize() int64 {
	var size int64
	for _, message := range a.runtime.OldMessages {
		if n := a.calculateContextSize(message); n > 0 {
			size = n
			continue
		}
		size += int64(len(message.Content) + 16)
		for _, call := range message.ToolCalls {
			size += int64(len(call.Arguments) + len(call.Name) + 16)
		}
	}
	return size
}

func (a *Agent) runLoop() error {

	// 重建
	if err := a.recoverConversation(); err != nil {
		return err
	}
	if a.runtime.tasks.Len() != 0 {
		if err := a.waitToolResult(); err != nil {
			return err
		}
	}

	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(a.base.Tools))
	for _, item := range a.base.Tools {
		tools = append(tools, item.Define())
	}

	contextLimit := int64(0)
	if a.base.Provider.Capabilities != nil {
		contextLimit = a.base.Provider.Capabilities.ContextLimit * 3 / 5
	}
	maxTurns := a.base.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 128
	}

	for turn := 0; turn < maxTurns; turn++ {
		if err := a.ctx.Err(); err != nil {
			return err
		}
		a.mergeMessages()
		if contextLimit > 0 && a.contextSize() >= contextLimit {
			if err := a.compact(); err != nil {
				return err
			}
		}

		response, err := a.base.Provider.Stream(
			a.ctx,
			a.prompt,
			a.runtime.OldMessages,
			&provider.ChatConfig{
				Tool: tools,
			},
			func(chunk *provider.StreamChunk, err error) error {
				if err != nil {
					return a.emit(&notify.ResetEvent{Error: err.Error()})
				} else {
					switch chunk.Type {
					case provider.Reasoning:
						return a.emit(&notify.ReasoningEvent{Content: chunk.Content})
					case provider.Completion:
						return a.emit(&notify.CompletionEvent{Content: chunk.Content})
					}
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
		a.runtime.OldMessages = append(a.runtime.OldMessages, response)

		if len(response.ToolCalls) != 0 {
			// 需要调用工具
			// 添加记录
			toolCalls := make([]*schema.ActiveTool, 0, len(response.ToolCalls))
			for _, toolCall := range response.ToolCalls {
				toolCalls = append(toolCalls, &schema.ActiveTool{
					ToolCallId: toolCall.ID,
					Name:       toolCall.Name,
					Arguments:  toolCall.Arguments,
				})
			}
			id, err := a.base.Store.AddMessageWithToolCalls(a.provider2StoreMessage(response), toolCalls...)
			if err != nil {
				return err
			}

			for _, toolCall := range toolCalls {
				err = a.toolExecute(id, toolCall.ToolCallId, toolCall.Name, toolCall.Arguments)
				if err != nil {
					return err
				}
			}
		} else {
			// 没有工具调用，结束对话
			// 会话的状态在service中统一处理
			message := a.provider2StoreMessage(response)
			message.Conversation = a.id
			return a.base.Store.AddConversationMessage(message)
		}

		if a.runtime.tasks.Len() != 0 {
			err = a.waitToolResult()
			if err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("agent exceeded %d model turns", maxTurns)
}

func (a *Agent) recoverConversation() error {
	conversation, err := a.base.Store.GetConversation(bson.M{
		"_id":    a.id,
		"status": schema.ConversationActive,
	})
	if err != nil {
		return err
	}

	if len(conversation.ActiveTools) == 0 {
		return nil
	}

	for _, toolCall := range conversation.ActiveTools {
		// 先检查异步任务
		// 异步任务不存在，则重新调用
		// 异步任务存在，则置入堆中，等待后续检查
		remoteTask, err := a.base.Store.GetRemoteTaskStore(bson.M{
			"conversation":      a.id,
			"tool.tool_call_id": toolCall.ToolCallId,
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		if remoteTask == nil {
			// 新建
			err = a.toolExecute(toolCall.Message, toolCall.ToolCallId, toolCall.Name, toolCall.Arguments)
			if err != nil {
				return err
			}
		} else {
			if t, ok := a.toolMap[toolCall.Name].(*subAgentTool); ok {
				if _, err := t.lookup(a.ctx, remoteTask.ContextId, remoteTask.TaskId); err != nil {
					return err
				}
			}
			// 恢复
			// 不需要调用，只需要定时获取结果即可
			a.taskRequeue(&taskheap.TaskItem{
				ContextId:  remoteTask.ContextId,
				TaskId:     remoteTask.TaskId,
				ToolCallId: remoteTask.Tool.ToolCallId,
				ToolName:   remoteTask.Tool.Name,
			})
		}
	}

	return nil
}

// 返回 error 时，表示无法处理的错误，应该结束对话
// 错误原因大致为：
// 1. 数据库错误；
// 2. 手动取消
func (a *Agent) toolExecute(
	messageId bson.ObjectID,
	toolCallId string,
	name string,
	arguments string,
) error {
	t, ok := a.toolMap[name]
	if !ok {
		err := a.toolFinished("", toolCallId, "tool not exists")
		if err != nil {
			return err
		}
		return nil
	}

	// A transport error does not prove a side-effecting tool was not executed.
	// Do not automatically replay Execute without an idempotency contract.
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	ctx = context.WithValue(ctx, subAgentCallKey{}, subAgentCall{message: messageId, id: toolCallId})
	result, err := t.Execute(ctx, a.base.User.Token, arguments)

	if err != nil {
		if a.ctx.Err() != nil {
			return a.ctx.Err()
		}
		if _, local := t.(*subAgentTool); local {
			return err
		}
		// 远程工具暂时不可用，标记为结束状态
		wErr := a.toolFinished("", toolCallId, err.Error())
		if wErr != nil {
			return wErr
		}
		return nil
	}

	if result == nil {
		return errors.New("tool returned no result")
	}
	if result.Status == tool.TaskCompleted || result.Status == tool.TaskFailed ||
		result.Status == tool.TaskCanceled || result.Status == tool.TaskRejected ||
		result.Status == tool.TaskAuthRequired {

		return a.toolFinished("", toolCallId, result.Content)
	}
	if result.TaskId != "" {
		// Subagent creation persists the parent link atomically before starting.
		if _, local := t.(*subAgentTool); local {
			a.taskRequeue(&taskheap.TaskItem{ContextId: result.ContextId, TaskId: result.TaskId, ToolCallId: toolCallId, ToolName: name})
			return nil
		}
		// 异步任务
		// 添加记录
		err = a.base.Store.CreateRemoteTaskStore(&schema.RemoteTaskStore{
			ContextId: result.ContextId,
			TaskId:    result.TaskId,
			Status:    schema.RemoteTaskSubmitted,
			Tool: &schema.TaskTool{
				ToolCallId: toolCallId,
				Name:       name,
			},
			Conversation: a.id,
			Message:      messageId,
		})
		if err != nil {
			return err
		}
		a.taskRequeue(&taskheap.TaskItem{
			ContextId:  result.ContextId,
			TaskId:     result.TaskId,
			ToolCallId: toolCallId,
			ToolName:   name,
		})
	} else {
		// 简单任务
		err = a.toolFinished("", toolCallId, result.Content)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) toolFinished(taskId, toolCallId, content string) error {
	latestId, err := a.base.Store.ActiveTaskFinished(a.id, &schema.FinishActiveReq{
		TaskId:     taskId,
		ToolCallId: toolCallId,
		Content:    content,
	})
	if err != nil {
		return err
	}
	a.runtime.latestId = latestId
	a.runtime.NewMessages = append(a.runtime.NewMessages, &provider.Message{
		Role:       provider.RoleTool,
		Content:    content,
		ToolCallId: toolCallId,
	})

	return nil
}

func (a *Agent) toolCheck(tasks ...*taskheap.TaskItem) error {
	if len(tasks) == 0 {
		return nil
	}

	taskIds := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIds = append(taskIds, task.TaskId)
	}
	objs, err := a.base.Store.ListRemoteTaskStores(bson.M{
		"conversation": a.id,
		"task_id":      bson.M{"$in": taskIds},
	})
	if err != nil {
		return err
	}

	var resolveItems []*resolver.InputRequiredItem

	for _, obj := range objs {
		if obj.Tool == nil {
			return errors.New("remote task has no tool metadata")
		}
		switch obj.Status {
		case schema.RemoteTaskSubmitted:
			t := a.toolMap[obj.Tool.Name]
			if t == nil {
				return fmt.Errorf("task tool %q is unavailable", obj.Tool.Name)
			}
			checkCtx, cancel := context.WithTimeout(a.ctx, time.Minute)
			result, err := t.Check(checkCtx, a.base.User.Token, obj.ContextId, obj.TaskId)
			cancel()
			if err == nil && result == nil {
				err = errors.New("tool returned no task state")
			}
			if err != nil {
				if _, local := t.(*subAgentTool); local {
					return err
				}
				// 请求错误，重新入队
				a.taskRequeue(&taskheap.TaskItem{
					ContextId:  obj.ContextId,
					TaskId:     obj.TaskId,
					ToolCallId: obj.Tool.ToolCallId,
					ToolName:   obj.Tool.Name,
				})
				continue
			}
			switch result.Status {
			case tool.TaskSubmitted, tool.TaskWorking:
				a.taskRequeue(&taskheap.TaskItem{
					ContextId:  obj.ContextId,
					TaskId:     obj.TaskId,
					ToolCallId: obj.Tool.ToolCallId,
					ToolName:   obj.Tool.Name,
				})
			case tool.TaskInputRequired:
				var inputSchema map[string]any
				function := t.Define().GetFunction()
				if function != nil {
					inputSchema = function.Parameters
				}
				builtMessage, _ := resolver.BuildInputRequiredMessage(obj.Tool.ToolCallId, result.Content, inputSchema)
				resolveItems = append(resolveItems, &resolver.InputRequiredItem{
					ContextId:  obj.ContextId,
					TaskId:     obj.TaskId,
					ToolCallId: obj.Tool.ToolCallId,
					ToolName:   obj.Tool.Name,
					Message:    builtMessage,
				})
			case tool.TaskCompleted, tool.TaskFailed, tool.TaskCanceled, tool.TaskRejected, tool.TaskAuthRequired:
				err = a.toolFinished(obj.TaskId, obj.Tool.ToolCallId, result.Content)
				if err != nil {
					return err
				}
			}
		case schema.RemoteTaskWaitingInput:
			a.taskRequeue(&taskheap.TaskItem{
				ContextId:  obj.ContextId,
				TaskId:     obj.TaskId,
				ToolCallId: obj.Tool.ToolCallId,
				ToolName:   obj.Tool.Name,
			})
		case schema.RemoteTaskInputted:
			err = a.toolResume(obj.ContextId, obj.TaskId, obj.Tool.Name, obj.Tool.ToolCallId, obj.Content)
			if err != nil {
				return err
			}
		case schema.RemoteTaskDone:
			if err := a.toolFinished(obj.TaskId, obj.Tool.ToolCallId, obj.Content); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown remote task status %q", obj.Status)
		}
	}

	err = a.resolveInputRequired(resolveItems...)
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) toolResume(contextId, taskId, toolName, toolCallId, content string) error {
	t := a.toolMap[toolName]
	if t == nil {
		return fmt.Errorf("task tool %q is unavailable", toolName)
	}
	resumeCtx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	result, err := t.Resume(resumeCtx, a.base.User.Token, contextId, taskId, content)
	if err != nil {
		a.taskRequeue(&taskheap.TaskItem{
			ContextId:  contextId,
			TaskId:     taskId,
			ToolCallId: toolCallId,
			ToolName:   toolName,
		})
		return nil
	}

	if result == nil {
		return errors.New("tool resume returned no result")
	}
	if result.TaskId == "" || result.Status == tool.TaskCompleted || result.Status == tool.TaskFailed ||
		result.Status == tool.TaskCanceled || result.Status == tool.TaskRejected ||
		result.Status == tool.TaskAuthRequired {
		return a.toolFinished(taskId, toolCallId, result.Content)
	}
	err = a.base.Store.UpdateRemoteTaskStore(
		bson.M{"conversation": a.id, "context_id": contextId, "task_id": taskId},
		bson.M{"$set": bson.M{"status": schema.RemoteTaskSubmitted}},
	)
	if err != nil {
		return err
	}

	a.taskRequeue(&taskheap.TaskItem{
		ContextId:  contextId,
		TaskId:     taskId,
		ToolCallId: toolCallId,
		ToolName:   toolName,
	})

	return nil
}

func (a *Agent) taskRequeue(item *taskheap.TaskItem) {
	item.Exp = time.Now().Add(time.Second * 15)
	a.runtime.tasks.Add(item)
}

func (a *Agent) waitToolResult() error {
	// The runner context bounds the total lifetime, including input waits.
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case now := <-ticker.C:
			tasks := a.runtime.tasks.PopExpired(now)
			err := a.toolCheck(tasks...)
			if err != nil {
				return err
			}
		}

		if a.runtime.tasks.Len() == 0 {
			return nil
		}
	}
}
