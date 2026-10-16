package service

import (
	"strconv"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Service) GetSessionList(user string, offset string, limit int) ([]*schema.Session, error) {
	filter := bson.M{
		"user":  user,
		"state": bson.M{"$ne": schema.SessionTemp},
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
		User:       user,
		State:      schema.SessionTemp,
		MessageSeq: 1,
		CreatedAt:  time.Now(),
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
		return nil, mongo.ErrNoDocuments
	}

	return objs[0], nil
}

func (s *Service) Stream(c *gin.Context, id string, seq uint64) error {
	ctx := c.Request.Context()

	consumer, err := s.rbs.AcquireConsumer(ctx, id, seq)
	if err != nil {
		return err
	}

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
	}
}
