package agent

import (
	"errors"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Execute
//
// 启动 subagent
func (a *Agent) Execute(content string) error {
	if a.parent.IsZero() {
		return errors.New("subagent has no parent agent")
	}

	if content == "" {
		// 宕机恢复
		// 从历史任务恢复
		task, err := a.store.GetTaskStoreServer(bson.M{"_id": a.id, "context_id": a.parent.Hex()})
		if err != nil {
			return err
		}
		if err = a.getHistoryMessages(); err != nil {
			return err
		}
	}
}

func (a *Agent) Resume(content string) error {
	// TODO
	panic("implement me")
}

func (a *Agent) GetState() (*schema.TaskStoreServer, error) {
	// TODO
	panic("implement me")
}
