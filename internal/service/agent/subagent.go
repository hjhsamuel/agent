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

	return errors.New("subagent execution is not implemented")
}

func (a *Agent) Resume(content string) error {
	// TODO
	return errors.New("subagent resume is not implemented")
}

func (a *Agent) GetState() (*schema.TaskStoreServer, error) {
	// TODO
	return nil, errors.New("subagent state is not implemented")
}
