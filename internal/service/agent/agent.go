package agent

import (
	"context"
	"errors"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/service/agent/prompts"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type AgentItf interface {
	ID() bson.ObjectID
	Wait()

	MainAgent
	SubAgent
}

// MainAgent
//
// main agent 方法
type MainAgent interface {
	Wait()
	// Start 开启会话
	Start(content string) error
}

// SubAgent
//
// subagent 方法
type SubAgent interface {
	// Execute 启动任务（创建新任务、宕机恢复）
	Execute(content string) error

	// Resume 继续任务，仅用于 input_required
	Resume(content string) error

	// GetState 获取任务状态
	GetState() (*schema.TaskStoreServer, error)
}

type Agent struct {
	id     bson.ObjectID
	parent bson.ObjectID
	ctx    context.Context

	prompt  string
	toolMap map[string]tool.Tool

	base    *BaseConfig
	runtime *Runtime
}

func (a *Agent) ID() bson.ObjectID {
	return a.id
}

func (a *Agent) Wait() {
	a.runtime.wg.Wait()
}

func (a *Agent) getHistoryMessages() error {
	// 获取记忆点
	compact, err := a.base.Store.GetConversationCompaction(
		bson.M{"conversation": a.id},
		options.FindOne().SetSort(bson.M{"_id": -1}),
	)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
	}

	// 获取记忆点之后的消息
	filter := bson.M{"conversation": a.id}
	if compact != nil {
		a.runtime.OldMessages = append(a.runtime.OldMessages, &provider.Message{
			Role:    provider.RoleSystem,
			Content: compact.Content,
		})
		filter["_id"] = bson.M{"$gt": compact.Message}
	}
	messages, err := a.base.Store.GetConversationMessages(
		filter,
		options.Find().SetSort(bson.M{"_id": 1}),
	)
	if err != nil {
		return err
	}

	for _, message := range messages {
		a.runtime.OldMessages = append(a.runtime.OldMessages, a.store2ProviderMessage(message))
		a.runtime.oldId = message.ID
	}

	return nil
}

func (a *Agent) store2ProviderMessage(message *schema.Message) *provider.Message {
	out := &provider.Message{
		Role:       message.Role,
		Content:    message.Content,
		ToolCallId: message.ToolCallId,
	}
	if len(message.ToolCalls) != 0 {
		for _, item := range message.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, &provider.ToolCall{
				ID:        item.ID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})
		}
	}
	if message.Usage != nil {
		out.Usage = &provider.TokenUsage{
			Total:       message.Usage.Total,
			Prompt:      message.Usage.Prompt,
			Cached:      message.Usage.Cached,
			Completions: message.Usage.Completions,
			Reasoning:   message.Usage.Reasoning,
		}
	}

	return out
}

func (a *Agent) provider2StoreMessage(message *provider.Message) *schema.Message {
	out := &schema.Message{
		Conversation: a.id,
		Role:         message.Role,
		Content:      message.Content,
		ToolCalls:    nil,
		ToolCallId:   message.ToolCallId,
		Usage:        nil,
	}
	if len(message.ToolCalls) != 0 {
		for _, item := range message.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, &schema.ToolCall{
				ID:        item.ID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})
		}
	}
	if message.Usage != nil {
		out.Usage = &schema.TokenUsage{
			Total:       message.Usage.Total,
			Prompt:      message.Usage.Prompt,
			Cached:      message.Usage.Cached,
			Completions: message.Usage.Completions,
			Reasoning:   message.Usage.Reasoning,
		}
	}

	return out
}

func NewAgent(
	ctx context.Context,
	conversationId bson.ObjectID,
	baseConfig *BaseConfig,
) AgentItf {
	mainCtx, mainCancel := context.WithCancel(ctx)

	toolMap := make(map[string]tool.Tool)
	for _, item := range baseConfig.Tools {
		name := item.Define().GetFunction().Name
		toolMap[name] = item
	}

	return &Agent{
		prompt:  prompts.GlobalSystemPrompt,
		ctx:     ctx,
		id:      conversationId,
		base:    baseConfig,
		toolMap: toolMap,
		runtime: &Runtime{
			OldMessages: make([]*provider.Message, 0),
			NewMessages: make([]*provider.Message, 0),
			tasks:       taskheap.NewManager(),
			main: &MainRuntime{
				ctx:    mainCtx,
				cancel: mainCancel,
			},
		},
	}
}
