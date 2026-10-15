package service

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/pkg/kms"
)

type Service struct {
	store *store.Store
	kms   *kms.KMS
	tool  *Tool // 全局可用的工具、subagent
	rbs   *ringbuffer.RingBufferManager

	life sync.Map
	wg   sync.WaitGroup
}

func (s *Service) setLife(id string, cancel context.CancelFunc) bool {
	_, loaded := s.life.LoadOrStore(id, cancel)
	return loaded
}

func (s *Service) lifeExists(id string) bool {
	_, ok := s.life.Load(id)
	return ok
}

func (s *Service) cancel(id string) {
	v, loaded := s.life.LoadAndDelete(id)
	if !loaded {
		return
	}

	v.(context.CancelFunc)()
}
