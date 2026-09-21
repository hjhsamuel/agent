package shard

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/service/agent"
)

type shard struct {
	lock   sync.RWMutex
	agents map[string]agent.MainAgent
	cancel map[string]context.CancelFunc
}

func (s *shard) Get(id string) (agent.MainAgent, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	out, ok := s.agents[id]
	return out, ok
}

func (s *shard) Set(cancel context.CancelFunc, id string, agent agent.MainAgent) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.agents[id] = agent
	s.cancel[id] = cancel
}

func (s *shard) Del(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.agents, id)
	if v, ok := s.cancel[id]; ok {
		v()
	}
	delete(s.cancel, id)
}

func newShard() *shard {
	return &shard{
		agents: make(map[string]agent.MainAgent),
		cancel: make(map[string]context.CancelFunc),
	}
}
