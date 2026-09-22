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
	closed bool
}

func (s *shard) Get(id string) (agent.MainAgent, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	out, ok := s.agents[id]
	return out, ok
}

func (s *shard) Set(cancel context.CancelFunc, id string, agent agent.MainAgent) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.closed {
		return false
	}
	if _, exists := s.agents[id]; exists {
		return false
	}
	s.agents[id] = agent
	s.cancel[id] = cancel
	return true
}

func (s *shard) Cancel(id string) bool {
	s.lock.RLock()
	cancel, ok := s.cancel[id]
	s.lock.RUnlock()
	if ok {
		cancel()
	}
	return ok
}

// Close seals the shard, detaches its runners and cancels outside the lock.
// The manager waits only after every shard has been canceled.
func (s *shard) Close() []agent.MainAgent {
	s.lock.Lock()
	s.closed = true
	runners := make([]agent.MainAgent, 0, len(s.agents))
	cancels := make([]context.CancelFunc, 0, len(s.cancel))
	for id, runner := range s.agents {
		runners = append(runners, runner)
		cancels = append(cancels, s.cancel[id])
	}
	clear(s.agents)
	clear(s.cancel)
	s.lock.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return runners
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
