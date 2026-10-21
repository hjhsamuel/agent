package ringbuffer

import (
	"container/heap"
	"context"
	"hash/maphash"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// RingBufferManager uses one maintenance goroutine. Push and readable Next calls
// do not acquire mutexes or read the clock. Shard locks protect registration,
// release and reclamation. Construct it with NewRingBufferManager; do not copy.
// Connected producers and consumers pin their buffers until Close or connection
// context cancellation. The last disconnect starts a one-minute retention lease.
// Only disconnected buffers enter expiry heaps; live connections need no renewal.
// Cleanup checks idle deadlines once per second and skips contended shards.
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
	if err := m.acquire(ctx, id, &p.lease, ringBufferProducerLease); err != nil {
		return nil, err
	}
	return p, nil
}

// AcquireConsumer grants the exclusive consumer connection until Close or ctx
// cancellation, rejecting an existing active consumer with ErrRingBufferConsumerBusy.
// It remains live without data, Next or Renew. ctx must be non-nil; use
// context.Background for an explicitly closed consumer. Close the old consumer
// before reconnecting with the last sequence received by the client.
// A retained sequence resumes at the following chunk; a missing
// sequence resumes at the newest retained SSEChunkTypeEnd, including the marker.
// If no marker is retained, Next waits for one. Zero starts at the oldest
// retained chunk.
// A disconnected consumer can be replaced while the buffer is retained.
func (m *RingBufferManager) AcquireConsumer(ctx context.Context, id string, lastSequence uint64) (*RingBufferConsumer, error) {
	if ctx == nil {
		return nil, ErrRingBufferInvalidLease
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c := &RingBufferConsumer{manager: m}
	if err := m.acquire(ctx, id, &c.lease, ringBufferConsumerLease); err != nil {
		return nil, err
	}
	c.sequence, c.reconnecting = lastSequence, lastSequence != 0
	return c, nil
}

func (m *RingBufferManager) acquire(ctx context.Context, id string, lease *ringBufferLease, kind ringBufferLeaseKind) error {
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
			// Cancellation callbacks run asynchronously. A reconnect may finish
			// releasing its canceled predecessor while already holding the lock.
			if m.checkLease(consumer) != nil {
				consumer.state.Store(ringBufferLeaseClosed)
				s.endLease(consumer, now)
			}
		}
		if b.idleIndex >= 0 && b.idleDeadline <= now {
			s.expire(b)
		}
		b = s.buffers[id]
	}
	if b == nil {
		b = &RingBuffer{shard: s, id: id, idleIndex: -1}
		if s.buffers == nil {
			s.buffers = make(map[string]*RingBuffer)
		}
		s.buffers[id] = b
	}
	if kind == ringBufferConsumerLease && b.consumer.Load() != nil {
		return ErrRingBufferConsumerBusy
	}
	if b.idleIndex >= 0 {
		heap.Remove(&s.expires, b.idleIndex)
	}
	lease.buffer, lease.kind = b, kind
	lease.canceled = ctx.Done()
	if kind == ringBufferConsumerLease {
		lease.wake, lease.done = make(chan struct{}, 1), make(chan struct{})
		b.consumer.Store(lease)
	} else {
		b.producers++
	}
	lease.next = s.leases
	if s.leases != nil {
		s.leases.prev = lease
	}
	s.leases = lease
	lease.stopCancel = context.AfterFunc(ctx, func() { _ = m.release(lease) })
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
	case SSEChunkTypeData, SSEChunkTypeReasoning, SSEChunkTypeHumanInTheLoop, SSEChunkTypeError:
	case SSEChunkTypeEnd:
		if len(chunk.Data) != 0 {
			return 0, ErrRingBufferInvalidChunk
		}
	default:
		return 0, ErrRingBufferInvalidChunk
	}
	chunk.Data = strings.Clone(chunk.Data)
	sequence, ok := lease.buffer.push(chunk, m.capacity)
	if !ok {
		if err := m.checkLease(lease); err != nil {
			return 0, err
		}
		return 0, ErrRingBufferLeaseClosed
	}
	if consumer := lease.buffer.consumer.Load(); consumer != nil && consumer.waiting.Load() {
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
	if lease.canceled != nil {
		select {
		case <-lease.canceled:
			return ErrRingBufferLeaseClosed
		default:
		}
	}
	return nil
}

func (m *RingBufferManager) renew(lease *ringBufferLease) error {
	if m == nil || lease.buffer == nil {
		return ErrRingBufferInvalidLease
	}
	return m.checkLease(lease)
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
	lease.state.Store(ringBufferLeaseClosed)
	s.endLease(lease, now)
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
					if m.closed.Load() {
						return
					}
					s := &m.shards[i]
					if s.mu.TryLock() {
						pending = s.reapExpired(now, ringBufferReapBatch) || pending
						s.mu.Unlock()
					}
				}
				if pending {
					// Drain bursts without an expiry-throughput cap, but let
					// registration and release run between bounded batches.
					runtime.Gosched()
				}
			}
		}
	}
}

func (m *RingBufferManager) clear() {
	for i := range m.shards {
		s := &m.shards[i]
		s.mu.Lock()
		for lease := s.leases; lease != nil; {
			next := lease.next
			lease.prev, lease.next = nil, nil
			lease.state.Store(ringBufferManagerClosed)
			if lease.stopCancel != nil {
				lease.stopCancel()
				lease.stopCancel = nil
			}
			if lease.done != nil {
				close(lease.done)
			}
			lease = next
		}
		for _, b := range s.buffers {
			b.idleIndex = -1
			b.clear()
			b.consumer.Store(nil)
			b.producers = 0
		}
		s.buffers, s.expires, s.leases = nil, nil, nil
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
