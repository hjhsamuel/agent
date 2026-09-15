package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const ConversationCollection = "agent_conversation"

type Conversation struct {
	ID        bson.ObjectID     `bson:"_id,omitempty"`
	Title     string            `bson:"title"`
	User      string            `bson:"user"`
	Status    ConversationState `bson:"status"`
	CreatedAt time.Time         `bson:"created_at"`
	UpdatedAt time.Time         `bson:"updated_at"`
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
	Role         MessageRole   `bson:"role"`
	Content      string        `bson:"content"`
	ToolCalls    []*ToolCall   `bson:"tool_calls"`
	ToolCallId   string        `bson:"tool_call_id"`
	CreatedAt    time.Time     `bson:"created_at"`
}

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type ToolCall struct {
	ID        string `bson:"id"`
	Name      string `bson:"name"`
	Arguments string `bson:"arguments"`
}

const CompactCollection = "agent_compact"

type Compaction struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Conversation bson.ObjectID `bson:"conversation"`
	Message      bson.ObjectID `bson:"message"`
	Content      string        `bson:"content"`
	CreatedAt    time.Time     `bson:"created_at"`
}
