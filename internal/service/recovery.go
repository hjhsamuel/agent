package service

import (
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) recoverSession(obj *schema.Session) error {
	if obj.State == schema.SessionWait {
		if !waitInputsReady(obj.Wait) {
			return nil // Leave unanswered confirmations waiting for the user.
		}
	}
	ctx, producer, err := s.acquireProducer(obj.ID.Hex())
	if err != nil {
		return err
	}
	msg, err := s.recoveryInput(ctx, obj)
	if err != nil {
		s.finishLife(obj.ID.Hex(), ctx)
		return err
	}
	r, err := s.buildRunner(obj)
	if err != nil {
		s.finishLife(obj.ID.Hex(), ctx)
		return err
	}

	if obj.State == schema.SessionWait {
		err = s.store.FinishWaitInput(obj.ID, confirmationMessages(obj)...)
	} else {
		_, err = s.store.FindOneAndUpdateSession(
			bson.M{"_id": obj.ID, "state": obj.State},
			bson.M{"$set": bson.M{"state": schema.SessionRunning, "updated_at": time.Now()}},
		)
	}
	if err != nil {
		s.finishLife(obj.ID.Hex(), ctx)
		return err
	}

	isFirst := obj.TurnSeq == 2 && obj.Name == "" && msg != nil && len(obj.Wait) == 0
	go s.newTurn(ctx, producer, obj.User, obj.ID, obj.TurnSeq-1, r, msg, isFirst)

	return nil
}
