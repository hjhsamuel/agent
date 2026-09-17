package schema

import (
	"time"

	"github.com/hjhsamuel/agent/pkg/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const ConversationCollection = "agent_conversation"

type Conversation struct {
	ID          bson.ObjectID     `bson:"_id,omitempty"`
	Title       string            `bson:"title"`
	User        string            `bson:"user"`
	Status      ConversationState `bson:"status"`
	ActiveTools []*ActiveTool     `bson:"active_tools"`
	CreatedAt   time.Time         `bson:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
}

type ActiveTool struct {
	ToolCallId string        `bson:"tool_call_id"`
	Message    bson.ObjectID `bson:"message"` // 触发调用的消息
	Name       string        `bson:"name"`
	Arguments  string        `bson:"arguments"`
}

type ConversationState string

const (
	ConversationTemp   ConversationState = "temp"   // 临时
	ConversationActive ConversationState = "active" // 运行中
	ConversationDone   ConversationState = "done"   // 对话结束
)

const MessageCollection = "agent_message"

type Message struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Conversation bson.ObjectID `bson:"conversation"`
	Role         provider.Role `bson:"role"`
	Content      string        `bson:"content"`
	ToolCalls    []*ToolCall   `bson:"tool_calls"`
	ToolCallId   string        `bson:"tool_call_id"`
	Usage        *TokenUsage   `bson:"usage"`
	CreatedAt    time.Time     `bson:"created_at"`
}

type ToolCall struct {
	ID        string `bson:"id"`
	Name      string `bson:"name"`
	Arguments string `bson:"arguments"`
}

type TokenUsage struct {
	Total       int64 `bson:"total"`
	Prompt      int64 `bson:"prompt"`
	Cached      int64 `bson:"cached"`
	Completions int64 `bson:"completions"`
	Reasoning   int64 `bson:"reasoning"`
}

const CompactCollection = "agent_compact"

type Compaction struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Conversation bson.ObjectID `bson:"conversation"`
	Message      bson.ObjectID `bson:"message"`
	Content      string        `bson:"content"`
	CreatedAt    time.Time     `bson:"created_at"`
}
