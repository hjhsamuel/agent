package agent

import (
	"context"
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/resolver"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/backoff"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/openai/openai-go/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (a *Agent) loopDefer() {
	if a.runtime.main != nil {
		a.runtime.main.cancel()
	}
	a.base.Exit <- a.id
}

func (a *Agent) calculateContextSize(message *provider.Message) int64 {
	return message.Usage.Prompt + message.Usage.Completions - message.Usage.Reasoning
}

func (a *Agent) loop() {
	defer a.runtime.wg.Done()
	defer a.loopDefer()

	// 重建
	if err := a.recoverConversation(); err != nil {
		return
	}

	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(a.base.Tools))
	for _, item := range a.base.Tools {
		tools = append(tools, item.Define())
	}

	var (
		contextLimit = int64(float64(a.base.Provider.Capabilities.ContextLimit) * 0.6)
		contextSize  int64
	)
	if len(a.runtime.OldMessages) != 0 {
		contextSize = a.calculateContextSize(a.runtime.OldMessages[len(a.runtime.OldMessages)-1])
	}

	for {
		if contextSize >= contextLimit {
			if err := a.compact(); err != nil {
				return
			}
		}

		a.runtime.OldMessages = append(a.runtime.OldMessages, a.runtime.NewMessages...)
		a.runtime.NewMessages = make([]*provider.Message, 0)
		if !a.runtime.latestId.IsZero() {
			a.runtime.oldId = a.runtime.latestId
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
					a.base.Up <- &notify.UpperEvent{
						ID:    a.id.Hex(),
						Event: &notify.ResetEvent{Error: err.Error()},
					}
				} else {
					switch chunk.Type {
					case provider.Reasoning:
						a.base.Up <- &notify.UpperEvent{
							ID:    a.id.Hex(),
							Event: &notify.ReasoningEvent{Content: chunk.Content},
						}
					case provider.Completion:
						a.base.Up <- &notify.UpperEvent{
							ID:    a.id.Hex(),
							Event: &notify.CompletionEvent{Content: chunk.Content},
						}
					}
				}
				return nil
			},
		)
		if err != nil {
			return
		}

		a.base.Up <- &notify.UpperEvent{
			ID:    a.id.Hex(),
			Event: &notify.DoneEvent{},
		}

		contextSize = a.calculateContextSize(response)
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
				return
			}

			for _, toolCall := range toolCalls {
				err = a.toolExecute(id, toolCall.ToolCallId, toolCall.Name, toolCall.Arguments)
				if err != nil {
					return
				}
			}
		} else {
			// 没有工具调用，结束对话
			// 会话的状态在service中统一处理
			message := a.provider2StoreMessage(response)
			message.Conversation = a.id
			_ = a.base.Store.AddConversationMessage(message)
			return
		}

		if a.runtime.tasks.Len() != 0 {
			err = a.waitToolResult()
			if err != nil {
				return
			}
		}
	}

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

	var (
		result *tool.ToolResult
		err    error
	)
	// 此处返回 error 表示发送远程请求失败
	// 在此处进行重试
	retry := backoff.NewBackoff(time.Second*3, time.Minute, 0.2)
	for attempt := 0; attempt < 8; attempt++ {
		result, err = t.Execute(a.ctx, a.base.User.Token, arguments)
		if err != nil {
			wErr := backoff.Wait(a.ctx, retry.Delay(attempt))
			if wErr != nil {
				return err
			}
			continue
		}
		break
	}

	if err != nil {
		// 远程工具暂时不可用，标记为结束状态
		wErr := a.toolFinished("", toolCallId, err.Error())
		if wErr != nil {
			return wErr
		}
		return nil
	}

	if result.TaskId != "" {
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
		switch obj.Status {
		case schema.RemoteTaskSubmitted:
			t := a.toolMap[obj.Tool.Name]
			result, err := t.Check(context.Background(), a.base.User.Token, obj.ContextId, obj.TaskId)
			if err != nil {
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
			// 正常不会进入
			continue
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
	_, err := t.Resume(context.Background(), a.base.User.Token, contextId, taskId, content)
	if err != nil {
		a.taskRequeue(&taskheap.TaskItem{
			ContextId:  contextId,
			TaskId:     taskId,
			ToolCallId: toolCallId,
			ToolName:   toolName,
		})
		return nil
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
	// TODO
	// 需要增加超长任务中断机制，避免因任务长时间运行导致 agent 死循环使资源无法释放
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
