package agent

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
)

type Runtime struct {
	wg sync.WaitGroup

	OldMessages []*provider.Message
	NewMessages []*provider.Message

	tasks *taskheap.Manager // 异步任务

	up chan<- *notify.UpperEvent // 上报的消息
	// TODO 暂时不做向下传递消息的机制，依靠 agent 主动扫描数据库

	main *MainRuntime // 专属于 main_agent 的运行时
}

type MainRuntime struct {
	ctx    context.Context
	cancel context.CancelFunc // 用于结束 ring buffer 续期

	done chan<- string // main agent 对话完成信号
}
