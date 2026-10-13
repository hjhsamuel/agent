package ringbuffer

import (
	"container/heap"
	"sync"
	"time"
)

type ringBufferShard struct {
	mu      sync.Mutex
	buffers map[string]*RingBuffer
	expires ringBufferLeaseHeap
}

func (s *ringBufferShard) endLease(lease *ringBufferLease, err error, idleDeadline time.Time) {
	heap.Remove(&s.expires, lease.index)
	lease.err = err
	b := lease.buffer
	if lease.kind == ringBufferConsumerLease {
		b.consumer = nil
		close(lease.wake)
	} else {
		b.producers--
	}
	if idleDeadline.After(b.idle.deadline) {
		b.idle.deadline = idleDeadline
	}
	if b.producers == 0 && b.consumer == nil {
		heap.Push(&s.expires, &b.idle)
	}
}

func (s *ringBufferShard) expire(lease *ringBufferLease) {
	if lease.kind == ringBufferIdleLease {
		heap.Remove(&s.expires, lease.index)
		b := lease.buffer
		delete(s.buffers, b.id)
		b.chunks = nil // Old lease handles must not retain payloads after removal.
		b.head, b.size = 0, 0
		if len(s.buffers) == 0 {
			s.buffers, s.expires = nil, nil
		}
		return
	}
	// An abandoned handle does not pin a buffer forever. If all handles expire,
	// their one-minute leases already supply the inactivity grace period.
	s.endLease(lease, ErrRingBufferLeaseExpired, lease.deadline)
	if b := lease.buffer; b.idle.index >= 0 && !lease.deadline.Before(b.idle.deadline) {
		s.expire(&b.idle)
	}
}

// reapExpired requires the shard lock and reports whether more entries are due.
func (s *ringBufferShard) reapExpired(now time.Time, limit int) bool {
	for n := 0; n < limit && len(s.expires) > 0 && !now.Before(s.expires[0].deadline); n++ {
		s.expire(s.expires[0])
	}
	return len(s.expires) > 0 && !now.Before(s.expires[0].deadline)
}
