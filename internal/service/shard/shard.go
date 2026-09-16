package shard

import (
	"sync"

	"github.com/hjhsamuel/agent/internal/service/agent"
)

type shard struct {
	lock   sync.RWMutex
	agents map[string]*agent.Agent
}

func (s *shard) Get(id string) (*agent.Agent, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	out, ok := s.agents[id]
	return out, ok
}

func (s *shard) Set(id string, agent *agent.Agent) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.agents[id] = agent
}

func (s *shard) Del(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.agents, id)
}

func newShard() *shard {
	return &shard{
		agents: make(map[string]*agent.Agent),
	}
}
