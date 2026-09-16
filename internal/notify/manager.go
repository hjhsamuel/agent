package notify

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/pkg"
	"github.com/hjhsamuel/agent/pkg/ringbuffer"
)

const ShardCount = 32

type Manager struct {
	mask  uint64
	slots []*shard

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (m *Manager) Start() {
	m.wg.Add(len(m.slots))
	for _, slot := range m.slots {
		go func(slot *shard) {
			defer m.wg.Done()
			slot.Do()
		}(slot)
	}
}

func (m *Manager) Close() {
	m.cancel()
	m.wg.Wait()
}

func (m *Manager) hash(id string) *shard {
	index := pkg.HashString(id) & m.mask
	return m.slots[index]
}

func (m *Manager) GetOrCreate(id string) *ringbuffer.RingBuffer[SSEvent] {
	slot := m.hash(id)
	return slot.GetOrCreate(id)
}

func (m *Manager) Touch(id string) {
	slot := m.hash(id)
	slot.Heartbeat(id)
}

func (m *Manager) PushEvent(id string, event SSEvent) {
	slot := m.hash(id)
	slot.PushEvent(id, event)
}

func (m *Manager) Consumer(id string, seq uint64) *ringbuffer.Consumer[SSEvent] {
	slot := m.hash(id)
	buffer := slot.GetOrCreate(id)
	return buffer.Subscribe(seq)
}

func NewManager(cnt int) *Manager {
	// 数量为 2 ^ n，使用位运算保证性能
	if cnt <= 0 || cnt&(cnt-1) != 0 {
		cnt = ShardCount
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		slots:  make([]*shard, cnt),
		mask:   uint64(cnt - 1),
		cancel: cancel,
	}
	for i := range m.slots {
		m.slots[i] = newShard(ctx, ringbuffer.Capacity)
	}

	return m
}
