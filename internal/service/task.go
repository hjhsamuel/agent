package service

import (
	"errors"
	"strconv"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/entities"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) ownedConversation(user *entities.UserInfo, id string) (bson.ObjectID, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return objectID, err
	}
	if user == nil {
		return objectID, errors.New("user is required")
	}
	_, err = s.store.WithContext(s.ctx).GetConversation(bson.M{"_id": objectID, "user": strconv.Itoa(user.ID)})
	return objectID, err
}

func (s *Service) ProvideInput(user *entities.UserInfo, id, contextID, taskID, content string) error {
	if content == "" {
		return errors.New("input is required")
	}
	if err := s.beginRequest(); err != nil {
		return err
	}
	defer s.requests.release()
	objectID, err := s.ownedConversation(user, id)
	if err != nil {
		return err
	}
	if _, err := s.agents.Get(objectID.Hex()); err != nil {
		return err
	}
	return s.store.WithContext(s.ctx).UpdateRemoteTaskStore(
		bson.M{
			"conversation": objectID,
			"context_id":   contextID,
			"task_id":      taskID,
			"status":       schema.RemoteTaskWaitingInput,
		},
		bson.M{
			"$set": bson.M{
				"status":  schema.RemoteTaskInputted,
				"content": content,
			},
		},
	)
}

// Cancel stops local execution and polling. Remote tool cancellation is not
// supported by Tool and must not be reported as having canceled remote work.
func (s *Service) Cancel(user *entities.UserInfo, id string) error {
	if err := s.beginRequest(); err != nil {
		return err
	}
	defer s.requests.release()
	objectID, err := s.ownedConversation(user, id)
	if err != nil {
		return err
	}
	if !s.agents.Cancel(objectID.Hex()) {
		return errors.New("conversation is not running")
	}
	return nil
}
