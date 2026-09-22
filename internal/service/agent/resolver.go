package agent

import (
	"encoding/json"
	"errors"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/compact"
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
	defer func() {
		for _, item := range items {
			a.taskRequeue(&taskheap.TaskItem{ContextId: item.ContextId, TaskId: item.TaskId, ToolCallId: item.ToolCallId, ToolName: item.ToolName})
		}
	}()

	var (
		messages = make([]*provider.Message, 0, len(items))
		itemMap  = make(map[string]*resolver.InputRequiredItem)
	)
	for _, item := range items {
		messages = append(messages, item.Message)
		itemMap[item.ToolCallId] = item
	}

	history := append(append([]*provider.Message(nil), a.runtime.OldMessages...), a.runtime.NewMessages...)
	transcript := compact.ConvertMessages("", history)[1].Content
	requests, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	response, err := a.base.Provider.Chat(
		a.ctx,
		prompts.InputRequiredResolverSystemPrompt,
		[]*provider.Message{{Role: provider.RoleUser, Content: transcript + "\nInput requests:\n" + string(requests)}},
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
		a.runtime.resolverFailures++
		if a.ctx.Err() != nil {
			return a.ctx.Err()
		}
		if a.runtime.resolverFailures >= 8 {
			return err
		}
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
		a.runtime.resolverFailures++
		if a.runtime.resolverFailures >= 8 {
			return err
		}
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
	processed := 0
	for _, decision := range decisions {
		if decision == nil {
			continue
		}
		item, ok := itemMap[decision.ToolCallId]
		if !ok {
			continue
		}

		switch decision.Action {
		case resolver.ProvideInput:
			if decision.Input == nil {
				continue
			}
			content, _ := json.Marshal(decision.Input)
			err = a.base.Store.UpdateRemoteTaskStore(
				bson.M{"conversation": a.id, "context_id": item.ContextId, "task_id": item.TaskId},
				bson.M{"$set": bson.M{"status": schema.RemoteTaskInputted, "content": string(content)}},
			)
		case resolver.AskUser:
			if decision.Question == "" {
				continue
			}
			err = a.base.Store.UpdateRemoteTaskStore(
				bson.M{"conversation": a.id, "context_id": item.ContextId, "task_id": item.TaskId},
				bson.M{"$set": bson.M{"status": schema.RemoteTaskWaitingInput, "content": decision.Question}},
			)
			events = append(events, &notify.InputRequiredItem{
				ContextId: item.ContextId,
				TaskId:    item.TaskId,
				Content:   decision.Question,
			})
		default:
			continue
		}
		if err != nil {
			return err
		}
		delete(itemMap, decision.ToolCallId)
		processed++
		a.taskRequeue(&taskheap.TaskItem{
			ContextId:  item.ContextId,
			TaskId:     item.TaskId,
			ToolCallId: item.ToolCallId,
			ToolName:   item.ToolName,
		})
	}

	if processed == 0 {
		a.runtime.resolverFailures++
		if a.runtime.resolverFailures >= 8 {
			return errors.New("resolver repeatedly returned no usable decisions")
		}
	} else {
		a.runtime.resolverFailures = 0
	}
	if len(events) != 0 {
		return a.emit(&notify.InputRequiredEvent{Events: events})
	}

	return nil
}
