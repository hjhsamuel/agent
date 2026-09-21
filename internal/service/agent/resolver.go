package agent

import (
	"context"
	"encoding/json"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/prompts"
	"github.com/hjhsamuel/agent/internal/service/agent/resolver"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (a *Agent) resolveInputRequired(items ...*resolver.InputRequiredItem) error {
	if len(items) == 0 {
		return nil
	}

	var (
		messages = make([]*provider.Message, 0, len(items))
		itemMap  = make(map[string]*resolver.InputRequiredItem)
	)
	for _, item := range items {
		messages = append(messages, item.Message)
		itemMap[item.ToolCallId] = item
	}

	response, err := a.base.Provider.Chat(
		context.Background(),
		prompts.InputRequiredResolverSystemPrompt,
		append(a.runtime.OldMessages, messages...),
		&provider.ChatConfig{
			Temperature: 0,
			ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{
					Type: "json_object",
				},
			},
		},
	)
	if err != nil {
		// 请求失败，等待下次处理
		for _, item := range items {
			a.taskRequeue(&taskheap.TaskItem{
				ContextId:  item.ContextId,
				TaskId:     item.TaskId,
				ToolCallId: item.ToolCallId,
				ToolName:   item.ToolName,
			})
		}
		return nil
	}

	decisions, err := resolver.ParseInputRequiredDecision(response.Content)
	if err != nil {
		// 参数解析失败，等待下次处理
		for _, item := range items {
			a.taskRequeue(&taskheap.TaskItem{
				ContextId:  item.ContextId,
				TaskId:     item.TaskId,
				ToolCallId: item.ToolCallId,
				ToolName:   item.ToolName,
			})
		}
		return nil
	}

	var events []*notify.InputRequiredItem
	for _, decision := range decisions {
		item, ok := itemMap[decision.ToolCallId]
		if ok {
			delete(itemMap, decision.ToolCallId)
		}

		switch decision.Action {
		case resolver.ProvideInput:
			content, _ := json.Marshal(decision.Input)
			_ = a.base.Store.UpdateRemoteTaskStore(
				bson.M{"conversation": a.id, "context_id": item.ContextId, "task_id": item.TaskId},
				bson.M{"$set": bson.M{"status": schema.RemoteTaskInputted, "content": content}},
			)
		case resolver.AskUser:
			_ = a.base.Store.UpdateRemoteTaskStore(
				bson.M{"conversation": a.id, "context_id": item.ContextId, "task_id": item.TaskId},
				bson.M{"$set": bson.M{"status": schema.RemoteTaskWaitingInput, "content": decision.Question}},
			)
			events = append(events, &notify.InputRequiredItem{
				ContextId: item.ContextId,
				TaskId:    item.TaskId,
				Content:   decision.Question,
			})
		}
		a.taskRequeue(&taskheap.TaskItem{
			ContextId:  item.ContextId,
			TaskId:     item.TaskId,
			ToolCallId: item.ToolCallId,
			ToolName:   item.ToolName,
		})
	}

	if len(events) != 0 {
		a.base.Up <- &notify.UpperEvent{
			ID:    a.id.Hex(),
			IsSub: false,
			Event: &notify.InputRequiredEvent{
				Events: events,
			},
			Heartbeat: false,
		}
	}

	return nil
}
