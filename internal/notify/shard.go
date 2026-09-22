package notify

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/hjhsamuel/agent/pkg/ringbuffer"
)

type shard struct {
	lock sync.RWMutex

	capacity uint64
	buffers  map[string]*bufferItem[SSEvent]

	heap bufferHeap[SSEvent]
	ttl  time.Duration

	ctx       context.Context
	events    chan *UpperEvent
	heartbeat chan string
}

func (s *shard) Heartbeat(id string) {
	select {
	case s.heartbeat <- id:
	default:

	}
}

func (s *shard) PushEvent(id string, event SSEvent) {
	select {
	case s.events <- &UpperEvent{ID: id, Event: event}:
	case <-s.ctx.Done():
	}
}

func (s *shard) Do() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.popExpired(now)
		case event := <-s.events:
			buffer, ok := s.get(event.ID)
			if !ok {
				continue
			}
			buffer.Push(event.Event)
		case id := <-s.heartbeat:
			s.touch(id)
		}
	}
}

func (s *shard) get(id string) (*ringbuffer.RingBuffer[SSEvent], bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	buffer, ok := s.buffers[id]
	if !ok {
		return nil, false
	}
	return buffer.buffer, true
}

func (s *shard) GetOrCreate(id string) *ringbuffer.RingBuffer[SSEvent] {
	if buffer, ok := s.get(id); ok {
		return buffer
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// 重复检查一次
	if buffer, ok := s.buffers[id]; ok {
		return buffer.buffer
	}

	buffer, _ := ringbuffer.NewRingBuffer[SSEvent](s.capacity)

	item := &bufferItem[SSEvent]{
		id:     id,
		active: time.Now(),
		buffer: buffer,
	}
	s.buffers[id] = item
	heap.Push(&s.heap, item)

	return buffer
}

func (s *shard) touch(id string) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	item, ok := s.buffers[id]
	if !ok {
		return false
	}

	item.active = time.Now().Add(time.Minute)
	heap.Fix(&s.heap, item.index)

	return true
}

func (s *shard) popExpired(t time.Time) ([]*ringbuffer.RingBuffer[SSEvent], bool) {
	s.lock.RLock()
	hasExpired := s.checkExpired(t)
	s.lock.RUnlock()
	if !hasExpired {
		return nil, false
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	var expired []*ringbuffer.RingBuffer[SSEvent]
	for s.checkExpired(t) {
		item := heap.Pop(&s.heap).(*bufferItem[SSEvent])
		delete(s.buffers, item.id)
		expired = append(expired, item.buffer)
	}

	return expired, len(expired) != 0
}

func (s *shard) checkExpired(t time.Time) bool {
	if len(s.heap) == 0 {
		return false
	}

	item := s.heap[0]
	if !item.active.Add(s.ttl).Before(t) {
		return false
	}
	return true
}

func newShard(ctx context.Context, capacity uint64) *shard {
	m := &shard{
		capacity:  capacity,
		buffers:   make(map[string]*bufferItem[SSEvent]),
		heap:      make(bufferHeap[SSEvent], 0),
		ttl:       time.Second * 30,
		events:    make(chan *UpperEvent, 1024),
		heartbeat: make(chan string, 32),
		ctx:       ctx,
	}

	heap.Init(&m.heap)

	return m
}
