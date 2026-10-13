package ringbuffer

import (
	"bytes"
	"container/heap"
	"hash/maphash"
	"sync/atomic"
	"time"
)

// RingBufferManager must be constructed with NewRingBufferManager and must not
// be copied. It uses sharded locks and expiry heaps, with one reaper goroutine
// regardless of buffer count. Close stops the reaper and releases stored chunks.
// After the last Close, a buffer retains its backlog for one minute. Abandoned
// leases expire one minute after their last renewal. Cleanup runs every second.
type RingBufferManager struct {
	capacity int
	seed     maphash.Seed
	shards   [ringBufferShardCount]ringBufferShard
	closed   atomic.Bool
	stop     chan struct{}
	done     chan struct{}
}

// NewRingBufferManager sets the maximum number of retained chunks per buffer.
// A non-positive capacity uses DefaultRingBufferCapacity. Payloads are copied
// on Push, so retained byte usage also depends on the size of incoming chunks.
func NewRingBufferManager() *RingBufferManager {
	m := &RingBufferManager{
		capacity: DefaultRingBufferCapacity,
		seed:     maphash.MakeSeed(),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go m.reap()
	return m
}

// AcquireProducer creates the buffer if necessary and acquires a producer lease.
func (m *RingBufferManager) AcquireProducer(id string) (*RingBufferProducer, error) {
	p := &RingBufferProducer{manager: m}
	if err := m.acquire(id, &p.lease, ringBufferProducerLease); err != nil {
		return nil, err
	}
	return p, nil
}

// AcquireConsumer creates the buffer if necessary. An unexpired consumer lease
// prevents another acquisition, even when that consumer is not currently reading.
func (m *RingBufferManager) AcquireConsumer(id string) (*RingBufferConsumer, error) {
	c := &RingBufferConsumer{manager: m}
	if err := m.acquire(id, &c.lease, ringBufferConsumerLease); err != nil {
		return nil, err
	}
	return c, nil
}

func (m *RingBufferManager) acquire(id string, lease *ringBufferLease, kind ringBufferLeaseKind) error {
	s := &m.shards[maphash.String(m.seed, id)%ringBufferShardCount]
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.closed.Load() {
		return ErrRingBufferManagerClosed
	}
	now := time.Now()
	b := s.buffers[id]
	if b != nil {
		// Check this buffer's idle/consumer deadline directly, without making
		// acquisitions scan the shard's backlog of unrelated expired leases.
		if b.consumer != nil && !now.Before(b.consumer.deadline) {
			s.expire(b.consumer)
		}
		if b.idle.index >= 0 && !now.Before(b.idle.deadline) {
			s.expire(&b.idle)
		}
		b = s.buffers[id]
	}
	if b == nil {
		b = &RingBuffer{shard: s, id: id}
		b.idle = ringBufferLease{buffer: b, kind: ringBufferIdleLease, index: -1}
		if s.buffers == nil {
			s.buffers = make(map[string]*RingBuffer)
		}
		s.buffers[id] = b
	}
	if kind == ringBufferConsumerLease && b.consumer != nil {
		return ErrRingBufferConsumerBusy
	}
	if b.idle.index >= 0 {
		heap.Remove(&s.expires, b.idle.index)
	}
	*lease = ringBufferLease{buffer: b, kind: kind, deadline: now.Add(RingBufferLeaseTTL), index: -1}
	b.idle.deadline = lease.deadline
	if kind == ringBufferConsumerLease {
		lease.wake = make(chan struct{}, 1)
		b.consumer = lease
	} else {
		b.producers++
	}
	heap.Push(&s.expires, lease)
	return nil
}

// Push copies data into the producer's buffer and renews its lease. It never
// waits for a consumer; on overflow it overwrites the oldest chunk.
func (m *RingBufferManager) Push(producer *RingBufferProducer, data []byte) (uint64, error) {
	if producer == nil || producer.manager != m || producer.lease.buffer == nil {
		return 0, ErrRingBufferInvalidLease
	}
	// Copy outside the shard lock so large SSE payloads do not hold it up.
	data = bytes.Clone(data)
	lease := &producer.lease
	s := lease.buffer.shard
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if err := m.checkLease(lease, now); err != nil {
		return 0, err
	}
	m.renewLocked(lease, now)
	b := lease.buffer
	b.push(data, m.capacity)
	if b.consumer != nil {
		select {
		case b.consumer.wake <- struct{}{}:
		default:
		}
	}
	return b.sequence, nil
}

func (m *RingBufferManager) renew(lease *ringBufferLease) error {
	if m == nil || lease.buffer == nil {
		return ErrRingBufferInvalidLease
	}
	s := lease.buffer.shard
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if err := m.checkLease(lease, now); err != nil {
		return err
	}
	m.renewLocked(lease, now)
	return nil
}

func (m *RingBufferManager) renewLocked(lease *ringBufferLease, now time.Time) {
	lease.deadline = now.Add(RingBufferLeaseTTL)
	lease.buffer.idle.deadline = lease.deadline
	heap.Fix(&lease.buffer.shard.expires, lease.index)
}

func (m *RingBufferManager) checkLease(lease *ringBufferLease, now time.Time) error {
	if m.closed.Load() {
		return ErrRingBufferManagerClosed
	}
	if lease.err != nil {
		return lease.err
	}
	if !now.Before(lease.deadline) {
		lease.buffer.shard.expire(lease)
		return ErrRingBufferLeaseExpired
	}
	return nil
}

func (m *RingBufferManager) release(lease *ringBufferLease) error {
	if m == nil || lease.buffer == nil {
		return ErrRingBufferInvalidLease
	}
	s := lease.buffer.shard
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.closed.Load() || lease.err != nil {
		return nil
	}
	now := time.Now()
	if !now.Before(lease.deadline) {
		s.expire(lease)
	} else {
		s.endLease(lease, ErrRingBufferLeaseClosed, now.Add(RingBufferLeaseTTL))
	}
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
		case now := <-ticker.C:
			// Work in bounded batches, releasing locks between batches. Only
			// expired heap entries are visited; active buffers are never scanned.
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
			lease.index, lease.err = -1, ErrRingBufferManagerClosed
			if lease.kind == ringBufferConsumerLease {
				close(lease.wake)
			}
		}
		for _, b := range s.buffers {
			b.chunks, b.consumer = nil, nil
			b.head, b.size, b.producers = 0, 0, 0
		}
		s.buffers, s.expires = nil, nil
		s.mu.Unlock()
	}
}

// Close is concurrent-safe and idempotent. It waits for cleanup to finish.
func (m *RingBufferManager) Close() error {
	if !m.closed.Swap(true) {
		close(m.stop)
	}
	<-m.done
	return nil
}
