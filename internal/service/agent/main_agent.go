package agent

import (
	"errors"
	"fmt"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/pkg/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Start
//
// 启动 agent
func (a *Agent) Start(content string) error {
	if !a.parent.IsZero() {
		return errors.New("is not main agent")
	}

	if content != "" {
		// 新对话
		// 修改会话状态
		err := a.store.UpdateConversation(
			bson.M{"_id": a.id},
			bson.M{"$set": bson.M{"status": schema.ConversationActive}},
		)
		if err != nil {
			return fmt.Errorf("start conversation error: %v", err)
		}
		// 添加当前消息
		err = a.store.AddConversationMessage(&schema.Message{
			Conversation: a.id,
			Role:         provider.RoleUser,
			Content:      content,
		})
		if err != nil {
			return fmt.Errorf("start conversation error: %v", err)
		}
	}

	// 加载完整对话
	if err := a.getHistoryMessages(); err != nil {
		return err
	}

	// 心跳
	a.runtime.wg.Add(1)
	go a.heartbeat()

	// 开始执行
	a.runtime.wg.Add(1)
	go a.loop()

	return nil
}

func (a *Agent) heartbeat() {
	defer a.runtime.wg.Done()

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-a.runtime.main.ctx.Done():
			return
		case <-ticker.C:
			hb := &notify.UpperEvent{
				ID:        a.id.Hex(),
				Heartbeat: true,
			}
			// 允许丢失
			select {
			case a.runtime.up <- hb:
			default:

			}
		}
	}
}
