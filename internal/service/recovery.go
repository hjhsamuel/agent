package service

import (
	"github.com/hjhsamuel/agent/internal/store/schema"
)

func (s *Service) recoverSession(obj *schema.Session) error {
	if obj.State != schema.SessionRunning && obj.State != schema.SessionInputReady {
		return nil
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

	isFirst := obj.TurnSeq == 2 && obj.Name == "" && msg != nil && len(obj.Wait) == 0
	go s.newTurn(ctx, producer, obj.User, obj.ID, obj.TurnSeq-1, r, msg, isFirst)

	return nil
}
