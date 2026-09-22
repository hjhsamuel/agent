package agent

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Start
//
// 启动 agent
func (a *Agent) Start(content string) error {
	if !a.parent.IsZero() {
		return errors.New("is not main agent")
	}
	m := a.runtime.main
	m.mu.Lock()
	if m.started || m.closed {
		m.mu.Unlock()
		return errors.New("main agent has already been started or closed")
	}
	m.started = true
	m.mu.Unlock()
	if err := a.ctx.Err(); err != nil {
		return err
	}

	if content != "" {
		// 新对话
		// 修改会话状态
		err := a.base.Store.BeginConversation(a.id, strconv.Itoa(a.base.User.ID), content)
		if err != nil {
			return fmt.Errorf("start conversation error: %v", err)
		}
	}

	// 加载完整对话
	if err := a.getHistoryMessages(); err != nil {
		_ = a.base.Store.UpdateConversation(bson.M{"_id": a.id}, bson.M{"$set": bson.M{"status": schema.ConversationFailed}})
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
			case a.base.Up <- hb:
			default:

			}
		}
	}
}
