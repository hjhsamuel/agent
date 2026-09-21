package agent

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/entities"
	"github.com/hjhsamuel/agent/internal/llm"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Runtime struct {
	wg sync.WaitGroup

	OldMessages []*provider.Message
	oldId       bson.ObjectID
	NewMessages []*provider.Message
	latestId    bson.ObjectID

	tasks *taskheap.Manager // 异步任务

	// TODO 暂时不做向下传递消息的机制，依靠 agent 主动扫描数据库

	main *MainRuntime // 专属于 main_agent 的运行时
}

type MainRuntime struct {
	ctx    context.Context
	cancel context.CancelFunc // 用于结束 ring buffer 续期
}

type BaseConfig struct {
	User *entities.UserInfo

	Skills   []*skill.Skill
	Tools    []tool.Tool
	Provider *llm.LLM
	Compact  *llm.LLM
	Store    *db.Dao

	Up   chan *notify.UpperEvent // 上报的消息
	Exit chan bson.ObjectID      // agent 退出信号
}
