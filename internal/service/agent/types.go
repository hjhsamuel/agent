package agent

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/entities"
	"github.com/hjhsamuel/agent/internal/llm"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Runtime struct {
	wg               sync.WaitGroup
	resolverFailures int

	OldMessages []*provider.Message
	oldId       bson.ObjectID
	NewMessages []*provider.Message
	latestId    bson.ObjectID

	tasks *taskheap.Manager // 异步任务

	// TODO 暂时不做向下传递消息的机制，依靠 agent 主动扫描数据库

	main *MainRuntime // 专属于 main_agent 的运行时
}

type MainRuntime struct {
	ctx      context.Context
	cancel   context.CancelFunc // 结束 main agent、心跳和所有 subagent
	mu       sync.Mutex
	children map[bson.ObjectID]*Agent
	closed   bool
	started  bool
}

type BaseConfig struct {
	User *entities.UserInfo

	Skills   []*skill.Skill
	Tools    []tool.Tool
	Provider *llm.LLM
	Compact  *llm.LLM
	Store    Store

	Up       chan *notify.UpperEvent // 上报的消息
	Shutdown <-chan struct{}
	MaxTurns int // Zero uses the default limit of 128 model turns.
}

// Store is the persistence contract used by an individual runner.
type Store interface {
	BeginConversation(bson.ObjectID, string, string) error
	GetConversation(bson.M) (*schema.Conversation, error)
	UpdateConversation(bson.M, bson.M) error
	GetConversationCompaction(bson.M, ...options.Lister[options.FindOneOptions]) (*schema.Compaction, error)
	GetConversationMessages(bson.M, ...options.Lister[options.FindOptions]) ([]*schema.Message, error)
	AddConversationMessage(...*schema.Message) error
	AddConversationCompaction(*schema.Compaction) error
	AddMessageWithToolCalls(*schema.Message, ...*schema.ActiveTool) (bson.ObjectID, error)
	ActiveTaskFinished(bson.ObjectID, ...*schema.FinishActiveReq) (bson.ObjectID, error)
	GetRemoteTaskStore(bson.M) (*schema.RemoteTaskStore, error)
	ListRemoteTaskStores(bson.M) ([]*schema.RemoteTaskStore, error)
	CreateRemoteTaskStore(*schema.RemoteTaskStore) error
	UpdateRemoteTaskStore(bson.M, bson.M) error
	CreateSubAgentTask(bson.ObjectID, bson.ObjectID, string, string) (bson.ObjectID, error)
	GetTaskStoreServer(bson.M) (*schema.TaskStoreServer, error)
	FinishSubAgentTask(bson.ObjectID, bson.ObjectID, tool.TaskStatus, string) error
	ResumeSubAgentTasks(bson.ObjectID, []*schema.RemoteTaskStore) error
}

var _ Store = (*db.Dao)(nil)
