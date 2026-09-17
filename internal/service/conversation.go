package service

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/pkg/ringbuffer"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) ListenSSE(
	c *gin.Context,
	userId string,
	id string,
	seq uint64,
) error {
	var (
		conversationId string
		err            error
		events         []*notify.InputRequiredItem
	)
	if id == "" {
		conversationId, err = s.store.CreateConversation(&schema.Conversation{
			User:   userId,
			Status: schema.ConversationTemp,
		})
		if err != nil {
			return err
		}
	} else {
		objectId, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
		objs, err := s.store.ListConversations(bson.M{"_id": objectId, "user": userId})
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			return errors.New("conversation not found")
		}

		conversationId = id

		// 检索会话是否存在需要用户输入的任务
		tasks, err := s.store.ListRemoteTaskStores(bson.M{
			"conversation": objectId,
			"status":       schema.RemoteTaskWaitingInput,
		})
		if err != nil {
			return err
		}
		for _, task := range tasks {
			events = append(events, &notify.InputRequiredItem{
				ContextId: task.ContextId,
				TaskId:    task.TaskId,
				Content:   task.Content,
			})
		}
	}

	consumer := s.notify.Consumer(conversationId, seq)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 创建缓存后回传会话id
	c.Render(-1, (&notify.InitEvent{ID: conversationId}).Event())
	c.Writer.Flush()

	if len(events) != 0 {
		c.Render(-1, (&notify.InputRequiredEvent{Events: events}).Event())
		c.Writer.Flush()
	}

	ticker := time.NewTimer(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-c.Done():
			return nil
		case <-consumer.Notify():
			if err := s.drainAndSendSSEvent(c, consumer); err != nil {
				return err
			}
		case <-ticker.C:
			s.notify.Touch(conversationId)
		}
	}
}

func (s *Service) drainAndSendSSEvent(
	c *gin.Context,
	consumer *ringbuffer.Consumer[notify.SSEvent],
) error {
	events, err := consumer.Drain()
	if err != nil {
		if errors.Is(err, ringbuffer.ErrCursorExpired) {
			e := err.(*ringbuffer.CursorExpiredError)
			event := &notify.ExpiredEvent{
				Oldest: e.OldestSeq,
				Latest: e.LatestSeq,
			}
			c.Render(-1, event.Event())
			c.Writer.Flush()
		}
		return err
	}

	for _, event := range events {
		out := event.Value.Event()
		out.Id = strconv.FormatUint(event.Seq, 10)
		c.Render(-1, out)
		c.Writer.Flush()
	}

	return nil
}
