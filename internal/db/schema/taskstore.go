package schema

import (
	"time"

	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const RemoteTaskStoreCollection = "agent_remote_task_store"

type RemoteTaskStore struct {
	ID           bson.ObjectID    `bson:"_id,omitempty"`
	ContextId    string           `bson:"context_id"`
	TaskId       string           `bson:"task_id"`
	Status       RemoteTaskStatus `bson:"status"`
	Content      string           `bson:"content"`
	Tool         *TaskTool        `bson:"tool"`
	Conversation bson.ObjectID    `bson:"conversation"`
	Message      bson.ObjectID    `bson:"message"`
	CreatedAt    time.Time        `bson:"created_at"`
	UpdatedAt    time.Time        `bson:"updated_at"`
}

type TaskTool struct {
	ToolCallId string `bson:"tool_call_id"`
	Name       string `bson:"name"`
}

type RemoteTaskStatus string

const (
	RemoteTaskSubmitted    RemoteTaskStatus = "submitted"     // 任务已成功提交
	RemoteTaskWaitingInput RemoteTaskStatus = "waiting_input" // input_required，等待输入
	RemoteTaskInputted     RemoteTaskStatus = "inputted"      // input_required，已获得输入
	RemoteTaskDone         RemoteTaskStatus = "done"          // 任务结束
)

const TaskStoreServerCollection = "agent_task_store_server"

type TaskStoreServer struct {
	ID        bson.ObjectID   `bson:"_id,omitempty"`
	ContextId string          `bson:"context_id"`
	TaskId    string          `bson:"task_id"`
	Status    tool.TaskStatus `bson:"status"`
	Artifacts []string        `bson:"artifacts"`
	History   []*TaskMessage  `bson:"history"`
	Version   int64           `bson:"version"`
	CreatedAt time.Time       `bson:"created_at"`
	UpdatedAt time.Time       `bson:"updated_at"`
}

type TaskMessage struct {
	Role    TaskMessageRole `bson:"role"`
	Content string          `bson:"content"`
}

type TaskMessageRole string

const (
	TaskRoleUser  TaskMessageRole = "user"  // client -> server
	TaskRoleAgent TaskMessageRole = "agent" // server -> client
)
