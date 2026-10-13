package service

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/pkg/kms"
)

type Service struct {
	store *store.Store
	kms   *kms.KMS
	tool  *Tool

	life sync.Map
}

func (s *Service) setLife(id string, cancel context.CancelFunc) {
	s.life.Store(id, cancel)
}

func (s *Service) cancel(id string) {
	v, loaded := s.life.LoadAndDelete(id)
	if !loaded {
		return
	}

	v.(context.CancelFunc)()
}
