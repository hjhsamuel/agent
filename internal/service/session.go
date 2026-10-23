package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/internal/service/prompts"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func (s *Service) GetSessionList(user string, offset string, limit int) ([]*schema.Session, error) {
	filter := bson.M{
		"user": user,
		"state": bson.M{"$in": []schema.SessionStatus{
			schema.SessionRunning,
			schema.SessionDone,
			schema.SessionWait,
		}},
	}
	if offset != "" {
		sessionId, err := bson.ObjectIDFromHex(offset)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": sessionId}
	}

	opts := options.Find().SetSort(bson.M{"_id": -1}).SetLimit(int64(limit))

	return s.store.FindSession(filter, opts)
}

func (s *Service) CreateSession(user string) (string, error) {
	id, err := s.store.CreateTempSession(&schema.Session{
		User:      user,
		State:     schema.SessionTemp,
		TurnSeq:   1,
		MsgSeq:    1,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return "", err
	}

	return id.Hex(), nil
}

func (s *Service) GetSession(user, id string) (*schema.Session, error) {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	objs, err := s.store.FindSession(bson.M{"_id": sessionId, "user": user})
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, nil
	}

	return objs[0], nil
}

func (s *Service) Stream(c *gin.Context, id string, seq uint64) error {
	ctx := c.Request.Context()

	consumer, err := s.rbs.AcquireConsumer(ctx, id, seq)
	if err != nil {
		return err
	}
	defer consumer.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	c.SSEvent(string(ringbuffer.SSEChunkTypeInit), id)
	c.Writer.Flush()

	for {
		chunks, err := consumer.Next(ctx)
		if err != nil {
			return err
		}
		for _, chunk := range chunks {
			c.Render(-1, sse.Event{
				Event: string(chunk.Type),
				Id:    strconv.FormatUint(chunk.Sequence, 10),
				Data:  chunk.Data,
			})
		}
		c.Writer.Flush()
	}
}

func (s *Service) ExtractSessionTopic(content string) (string, error) {
	_, llmModel, err := s.randomModel(schema.ChatModel)
	if err != nil {
		return "", err
	}

	response := llmModel.GenerateContent(
		context.Background(),
		&model.LLMRequest{
			Contents: []*genai.Content{
				{Role: "system", Parts: []*genai.Part{{Text: prompts.SessionTopicSystemPrompt}}},
				{Role: "user", Parts: []*genai.Part{{Text: content}}},
			},
		},
		false,
	)

	var b strings.Builder
	for item, err := range response {
		if err != nil {
			return "", err
		}
		if item == nil || item.Content == nil || len(item.Content.Parts) == 0 {
			continue
		}

		for _, elem := range item.Content.Parts {
			if elem == nil || elem.Text == "" {
				continue
			}
			b.WriteString(elem.Text)
		}
	}
	return b.String(), nil
}

func (s *Service) DeleteSession(user, id string) error {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	// 用户会话为软删除
	err = s.store.DeleteSession(user, sessionId)
	if err != nil {
		return err
	}
	// SessionService 为直接删除，忽略删除错误
	_ = s.adkSession.Delete(context.Background(), &session.DeleteRequest{
		AppName:   schema.RunnerName,
		UserID:    user,
		SessionID: id,
	})

	return nil
}

func (s *Service) GetSessionMessages(
	id string,
	offset int64,
	limit int,
) ([]*schema.Message, error) {
	sessionId, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"session": sessionId,
	}
	if offset != 0 {
		filter["seq"] = bson.M{"$lt": offset}
	}

	return s.store.GetMessages(
		filter,
		options.Find().SetSort(bson.M{"_id": -1}).SetLimit(int64(limit)),
	)
}
