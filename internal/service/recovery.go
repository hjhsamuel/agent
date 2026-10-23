package service

import (
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/service/prompts"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/genai"
)

func (s *Service) recoverSession(obj *schema.Session) error {
	r, err := s.getRunner(
		obj.User,
		&AgentConfig{
			Name:        schema.MainAgentName,
			Description: "main agent",
			Prompt:      prompts.SystemPrompt,
		},
	)
	if err != nil {
		return err
	}

	var msg *genai.Content
	if obj.State == schema.SessionWait {
		if len(obj.Wait) == 0 {
			return errors.New("missing submitted confirmation input")
		}
		for _, input := range obj.Wait {
			if input == nil || input.ID == "" || input.Content == "" {
				return errors.New("incomplete submitted confirmation input")
			}
		}
		msg = buildInputRequiredMessage(obj.Wait)
	}

	ctx, producer, err := s.acquireProducer(obj.ID.Hex())
	if err != nil {
		return err
	}

	_, err = s.store.FindOneAndUpdateSession(
		bson.M{"_id": obj.ID, "state": obj.State},
		bson.M{"$set": bson.M{"state": schema.SessionRunning, "updated_at": time.Now()}},
	)
	if err != nil {
		s.finishLife(obj.ID.Hex(), ctx)
		return err
	}

	s.wg.Add(1)
	go s.newTurn(ctx, producer, obj.User, obj.ID, obj.TurnSeq-1, r, msg, false)

	return nil
}
