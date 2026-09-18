package agent

import (
	"errors"

	"github.com/hjhsamuel/agent/internal/db/schema"
)

// Execute
//
// 启动 subagent
func (a *Agent) Execute(content string) error {
	// TODO
	if a.parent.IsZero() {
		return errors.New("subagent has no parent agent")
	}

	if content == "" {
		// 宕机恢复
		// 从历史任务恢复
	}

	return nil
}

func (a *Agent) Resume(content string) error {
	// TODO
	panic("implement me")
}

func (a *Agent) GetState() (*schema.TaskStoreServer, error) {
	// TODO
	panic("implement me")
}
