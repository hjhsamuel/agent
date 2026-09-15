package notify

import (
	"container/heap"
	"sync"
	"time"

	"github.com/hjhsamuel/agent/pkg/ringbuffer"
)

type Manager struct {
	lock sync.RWMutex

	capacity uint64
	buffers  map[string]*bufferItem[SSEvent]

	heap bufferHeap[SSEvent]
	ttl  time.Duration
}

func (m *Manager) Get(id string) (*ringbuffer.RingBuffer[SSEvent], bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	buffer, ok := m.buffers[id]
	return buffer.buffer, ok
}

func (m *Manager) GetOrCreate(id string) (*ringbuffer.RingBuffer[SSEvent], error) {
	if buffer, ok := m.Get(id); ok {
		return buffer, nil
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	// 重复检查一次
	if buffer, ok := m.buffers[id]; ok {
		return buffer.buffer, nil
	}

	buffer, err := ringbuffer.NewRingBuffer[SSEvent](m.capacity)
	if err != nil {
		return nil, err
	}

	item := &bufferItem[SSEvent]{
		id:     id,
		active: time.Now(),
		buffer: buffer,
	}
	m.buffers[id] = item
	m.heap.Push(item)

	return buffer, nil
}

func (m *Manager) Touch(id string, t time.Time) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	item, ok := m.buffers[id]
	if !ok {
		return false
	}

	item.active = t
	heap.Fix(&m.heap, item.index)

	return true
}

func (m *Manager) PopExpired(t time.Time) (*ringbuffer.RingBuffer[SSEvent], bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.heap) == 0 {
		return nil, false
	}

	item := m.heap[0]
	if !item.active.Add(m.ttl).Before(t) {
		return nil, false
	}

	heap.Pop(&m.heap)
	delete(m.buffers, item.id)

	return item.buffer, true
}

func NewManager(capacity uint64) *Manager {
	m := &Manager{
		capacity: capacity,
		buffers:  make(map[string]*bufferItem[SSEvent]),
		heap:     make(bufferHeap[SSEvent], 0),
		ttl:      time.Second * 30,
	}

	heap.Init(&m.heap)

	return m
}
