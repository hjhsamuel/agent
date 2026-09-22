package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/entities"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent"
	"github.com/hjhsamuel/agent/pkg/ringbuffer"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) ListenSSE(
	c *gin.Context,
	userId string,
	id string,
	seq uint64,
) error {
	if err := s.beginRequest(); err != nil {
		return err
	}
	defer s.requests.release()
	store := s.store.WithContext(c.Request.Context())
	var (
		conversationId string
		err            error
		events         []*notify.InputRequiredItem
	)
	if id == "" {
		conversationId, err = store.CreateConversation(&schema.Conversation{
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
		objs, err := store.ListConversations(bson.M{"_id": objectId, "user": userId})
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			return errors.New("conversation not found")
		}

		conversationId = objectId.Hex()

		// 检索会话是否存在需要用户输入的任务
		tasks, err := store.ListRemoteTaskStores(bson.M{
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
	if err := sendSSE(c, (&notify.InitEvent{ID: conversationId}).Event()); err != nil {
		return err
	}

	if len(events) != 0 {
		if err := sendSSE(c, (&notify.InputRequiredEvent{Events: events}).Event()); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	// Replay buffered events even if the old reader consumed the notification.
	if err := s.drainAndSendSSEvent(c, consumer); err != nil {
		return err
	}
	for {
		select {
		case <-s.ctx.Done():
			return nil
		case <-consumer.Done():
			return nil
		case <-c.Request.Context().Done():
			return nil
		case <-consumer.Notify():
			if err := s.drainAndSendSSEvent(c, consumer); err != nil {
				return err
			}
		case <-ticker.C:
			s.notify.Touch(conversationId)
			if err := sendSSE(c, sse.Event{Event: "heartbeat", Data: ""}); err != nil {
				return err
			}
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
			if writeErr := sendSSE(c, event.Event()); writeErr != nil {
				return writeErr
			}
		}
		return err
	}

	for _, event := range events {
		out := event.Value.Event()
		out.Id = strconv.FormatUint(event.Seq, 10)
		if err := sendSSE(c, out); err != nil {
			return err
		}
	}

	return nil
}

func sendSSE(c *gin.Context, event sse.Event) error {
	controller := http.NewResponseController(c.Writer)
	if err := controller.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if err := event.Render(c.Writer); err != nil {
		return err
	}
	return controller.Flush()
}

func (s *Service) Chat(user *entities.UserInfo, id string, content string) error {
	if user == nil || content == "" {
		return errors.New("user and content are required")
	}
	if err := s.beginRequest(); err != nil {
		return err
	}
	defer s.requests.release()
	conversationId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	id = conversationId.Hex()

	provider, err := s.providers.Get(schema.ChatModel)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(s.ctx, 24*time.Hour)
	runner := agent.NewAgent(
		ctx,
		conversationId,
		&agent.BaseConfig{
			User:     user,
			Skills:   nil,
			Tools:    s.tools.All(),
			Provider: provider,
			Compact:  provider,
			Store:    s.store.WithContext(ctx),
			Up:       s.events,
			Shutdown: s.ctx.Done(),
		},
	)
	if !s.agents.Set(cancel, id, runner) {
		cancel()
		return errors.New("conversation is already running")
	}
	if err := runner.Start(content); err != nil {
		s.agents.Delete(conversationId.Hex())
		cancel()
		return err
	}

	return nil
}

func (s *Service) agentEvent() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.events:
			if event.Finished {
				id, _ := bson.ObjectIDFromHex(event.ID)
				state := schema.ConversationDone
				reason := ""
				var terminal notify.SSEvent = &notify.DoneEvent{}
				if event.Error != nil {
					state = schema.ConversationFailed
					reason = event.Error.Error()
					terminal = &notify.ErrorEvent{Error: event.Error.Error()}
				}
				err := s.store.WithContext(s.ctx).FinishConversation(id, state, reason)
				if err != nil {
					terminal = &notify.ErrorEvent{Error: err.Error()}
				}
				s.notify.PushEvent(event.ID, terminal)
				s.agents.Delete(event.ID)
				continue
			}
			if event.Heartbeat {
				s.notify.Touch(event.ID)
			} else {
				s.notify.PushEvent(event.ID, event.Event)
			}
		}
	}
}
