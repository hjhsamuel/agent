package ringbuffer

import (
	"bytes"
	"container/heap"
	"context"
	"hash/maphash"
	"sync/atomic"
	"time"
)

// RingBufferManager uses one maintenance goroutine. Push, Next and Renew do not
// acquire mutexes or adjust expiry heaps. Shard locks protect registration,
// release and reclamation. Construct it with NewRingBufferManager; do not copy.
// Connected producers pin their buffers. The last explicit Close starts a
// one-minute retention lease; abandoned consumers expire after one minute
// without renewal. Cleanup checks deadlines once per second.
type RingBufferManager struct {
	capacity int
	seed     maphash.Seed
	start    time.Time
	shards   [ringBufferShardCount]ringBufferShard
	closed   atomic.Bool
	stop     chan struct{}
	done     chan struct{}
}

// NewRingBufferManager retains at most DefaultRingBufferCapacity chunks per
// buffer. Storage grows by segment; payload sizes determine retained byte use.
func NewRingBufferManager() *RingBufferManager {
	m := &RingBufferManager{
		capacity: DefaultRingBufferCapacity,
		seed:     maphash.MakeSeed(),
		start:    time.Now(),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go m.reap()
	return m
}

func (m *RingBufferManager) now() int64 { return int64(time.Since(m.start)) }

// AcquireProducer stays connected until Close or ctx cancellation, including
// during arbitrarily long Push pauses. It creates no per-producer timer or
// standing goroutine. ctx must be non-nil; use context.Background for an
// explicitly closed producer.
func (m *RingBufferManager) AcquireProducer(ctx context.Context, id string) (*RingBufferProducer, error) {
	if ctx == nil {
		return nil, ErrRingBufferInvalidLease
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p := &RingBufferProducer{manager: m}
	p.lease.ctx = ctx
	if err := m.acquire(id, &p.lease, ringBufferProducerLease); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = p.Close() })
	s := p.lease.buffer.shard
	s.mu.Lock()
	if ringBufferLeaseError(p.lease.state.Load()) == nil {
		p.lease.stopCancel = stop
	} else {
		stop()
	}
	s.mu.Unlock()
	return p, nil
}

// AcquireConsumer grants the exclusive one-minute consumer lease, rejecting an
// existing active consumer with ErrRingBufferConsumerBusy. Next and Renew extend
// it. Close the old consumer before reconnecting with the last sequence received
// by the client. A retained sequence resumes at the following chunk; a missing
// sequence resumes at the newest retained SSEChunkTypeEnd, including the marker.
// If no marker is retained, Next waits for one. Zero starts at the oldest
// retained chunk.
// A closed or expired consumer can be replaced while the buffer is retained.
func (m *RingBufferManager) AcquireConsumer(id string, lastSequence uint64) (*RingBufferConsumer, error) {
	c := &RingBufferConsumer{manager: m}
	if err := m.acquire(id, &c.lease, ringBufferConsumerLease); err != nil {
		return nil, err
	}
	c.sequence, c.reconnecting = lastSequence, lastSequence != 0
	return c, nil
}

func (m *RingBufferManager) acquire(id string, lease *ringBufferLease, kind ringBufferLeaseKind) error {
	s := &m.shards[maphash.String(m.seed, id)%ringBufferShardCount]
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.closed.Load() {
		return ErrRingBufferManagerClosed
	}
	now := m.now()
	b := s.buffers[id]
	if b != nil {
		if consumer := b.consumer.Load(); consumer != nil {
			s.expire(consumer, now)
		}
		if b.idle.index >= 0 && b.idle.deadline <= now {
			s.expire(&b.idle, now)
		}
		b = s.buffers[id]
	}
	if b == nil {
		b = &RingBuffer{shard: s, id: id}
		b.idle.buffer, b.idle.kind, b.idle.index = b, ringBufferIdleLease, -1
		if s.buffers == nil {
			s.buffers = make(map[string]*RingBuffer)
		}
		s.buffers[id] = b
	}
	if kind == ringBufferConsumerLease && b.consumer.Load() != nil {
		return ErrRingBufferConsumerBusy
	}
	if b.idle.index >= 0 {
		heap.Remove(&s.expires, b.idle.index)
	}
	lease.buffer, lease.kind = b, kind
	lease.deadline, lease.index = now+int64(RingBufferLeaseTTL), -1
	lease.state.Store(uint64(lease.deadline))
	ringBufferExtend(&b.retainUntil, lease.deadline)
	if kind == ringBufferConsumerLease {
		lease.wake, lease.done = make(chan struct{}, 1), make(chan struct{})
		b.consumer.Store(lease)
	} else {
		b.producers++
	}
	heap.Push(&s.expires, lease)
	return nil
}

// Push assigns Sequence, copies Data and publishes without locking. The input
// Sequence is ignored. End markers must have empty Data. Human-in-the-loop
// signals are delivered in order without pausing producers. Multiple producers
// may push concurrently; stalled writers cannot replace newer sequences. Overflow
// directly overwrites old chunks and never waits for the consumer.
func (m *RingBufferManager) Push(producer *RingBufferProducer, chunk SSEChunk) (uint64, error) {
	if producer == nil || producer.manager != m || producer.lease.buffer == nil {
		return 0, ErrRingBufferInvalidLease
	}
	lease := &producer.lease
	if err := m.checkLease(lease); err != nil {
		return 0, err
	}
	switch chunk.Type {
	case SSEChunkTypeData, SSEChunkTypeReasoning, SSEChunkTypeHumanInTheLoop:
	case SSEChunkTypeEnd:
		if len(chunk.Data) != 0 {
			return 0, ErrRingBufferInvalidChunk
		}
	default:
		return 0, ErrRingBufferInvalidChunk
	}
	chunk.Data = bytes.Clone(chunk.Data)
	sequence, ok := lease.buffer.push(chunk, m.capacity)
	if !ok {
		if err := m.checkLease(lease); err != nil {
			return 0, err
		}
		return 0, ErrRingBufferLeaseClosed
	}
	if consumer := lease.buffer.consumer.Load(); consumer != nil {
		select {
		case consumer.wake <- struct{}{}:
		default:
		}
	}
	return sequence, nil
}

func (m *RingBufferManager) checkLease(lease *ringBufferLease) error {
	if m.closed.Load() {
		return ErrRingBufferManagerClosed
	}
	state := lease.state.Load()
	if err := ringBufferLeaseError(state); err != nil {
		return err
	}
	if lease.kind == ringBufferProducerLease {
		if lease.ctx != nil && lease.ctx.Err() != nil {
			return ErrRingBufferLeaseClosed
		}
	} else if int64(state) <= m.now() {
		return ErrRingBufferLeaseExpired
	}
	return nil
}

func (m *RingBufferManager) renew(lease *ringBufferLease) error {
	if m == nil || lease.buffer == nil {
		return ErrRingBufferInvalidLease
	}
	for {
		if m.closed.Load() {
			return ErrRingBufferManagerClosed
		}
		state := lease.state.Load()
		if err := ringBufferLeaseError(state); err != nil {
			return err
		}
		if lease.ctx != nil && lease.ctx.Err() != nil {
			return ErrRingBufferLeaseClosed
		}
		now := m.now()
		if lease.kind == ringBufferConsumerLease && int64(state) <= now {
			return ErrRingBufferLeaseExpired
		}
		deadline := max(state, uint64(now+int64(RingBufferLeaseTTL)))
		if lease.state.CompareAndSwap(state, deadline) {
			ringBufferExtend(&lease.buffer.retainUntil, int64(deadline))
			return nil
		}
	}
}

func (m *RingBufferManager) release(lease *ringBufferLease) error {
	if m == nil || lease.buffer == nil {
		return ErrRingBufferInvalidLease
	}
	s := lease.buffer.shard
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.closed.Load() || ringBufferLeaseError(lease.state.Load()) != nil {
		return nil
	}
	now := m.now()
	// Explicit Close always wins against a concurrent renewal.
	lease.state.Store(ringBufferLeaseClosed)
	s.endLease(lease, now+int64(RingBufferLeaseTTL))
	return nil
}

func (m *RingBufferManager) reap() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer close(m.done)
	defer m.clear()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			now := m.now()
			for pending := true; pending; {
				pending = false
				for i := range m.shards {
					select {
					case <-m.stop:
						return
					default:
					}
					s := &m.shards[i]
					s.mu.Lock()
					pending = s.reapExpired(now, ringBufferReapBatch) || pending
					s.mu.Unlock()
				}
			}
		}
	}
}

func (m *RingBufferManager) clear() {
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		for _, lease := range s.expires {
			lease.index = -1
			lease.state.Store(ringBufferManagerClosed)
			if lease.stopCancel != nil {
				lease.stopCancel()
				lease.stopCancel = nil
			}
			if lease.done != nil {
				close(lease.done)
			}
		}
		for _, b := range s.buffers {
			b.clear()
			b.consumer.Store(nil)
			b.producers = 0
		}
		s.buffers, s.expires = nil, nil
		s.mu.Unlock()
	}
}

// Close is concurrent-safe and idempotent and waits for reclamation to finish.
func (m *RingBufferManager) Close() error {
	if !m.closed.Swap(true) {
		close(m.stop)
	}
	<-m.done
	return nil
}
