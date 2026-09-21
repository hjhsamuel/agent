package shard

import (
	"context"
	"errors"

	"github.com/hjhsamuel/agent/internal/service/agent"
	"github.com/hjhsamuel/agent/pkg"
)

const ShardCount = 64

type Manager struct {
	mask   uint64
	shards []*shard
}

func (m *Manager) Get(id string) (agent.MainAgent, error) {
	slot := m.hash(id)
	out, ok := slot.Get(id)
	if !ok {
		return nil, errors.New("agent not found")
	}
	return out, nil
}

func (m *Manager) hash(id string) *shard {
	index := pkg.HashString(id) & m.mask
	return m.shards[index]
}

func (m *Manager) Set(cancel context.CancelFunc, id string, agt agent.MainAgent) {
	slot := m.hash(id)
	slot.Set(cancel, id, agt)
}

func (m *Manager) Delete(id string) {
	slot := m.hash(id)
	slot.Del(id)
}

func NewManager(cnt int) *Manager {
	// 数量为 2 ^ n，使用位运算保证性能
	if cnt <= 0 || cnt&(cnt-1) != 0 {
		cnt = ShardCount
	}

	m := &Manager{
		shards: make([]*shard, cnt),
		mask:   uint64(cnt - 1),
	}
	for i := range m.shards {
		m.shards[i] = newShard()
	}

	return m
}
