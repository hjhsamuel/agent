package ringbuffer

import (
	"container/heap"
	"sync"
)

type ringBufferShard struct {
	mu      sync.Mutex
	buffers map[string]*RingBuffer
	leases  *ringBufferLease
	expires ringBufferHeap
}

// endLease requires the shard lock and an already-disconnected lease state.
func (s *ringBufferShard) endLease(lease *ringBufferLease, now int64) {
	if lease.prev != nil {
		lease.prev.next = lease.next
	} else {
		s.leases = lease.next
	}
	if lease.next != nil {
		lease.next.prev = lease.prev
	}
	lease.prev, lease.next = nil, nil
	if lease.stopCancel != nil {
		lease.stopCancel()
		lease.stopCancel = nil
	}
	b := lease.buffer
	if lease.kind == ringBufferConsumerLease {
		b.consumer.CompareAndSwap(lease, nil)
		close(lease.done)
	} else {
		b.producers--
	}
	if b.producers == 0 && b.consumer.Load() == nil {
		b.idleDeadline = now + int64(RingBufferLeaseTTL)
		heap.Push(&s.expires, b)
	}
}

// expire removes an idle buffer under the shard lock. Reconnecting removes it
// from the heap first, so live connections require no periodic maintenance.
func (s *ringBufferShard) expire(b *RingBuffer) {
	heap.Remove(&s.expires, b.idleIndex)
	delete(s.buffers, b.id)
	b.clear()
	if len(s.buffers) == 0 {
		s.buffers, s.expires = nil, nil
	}
}

// reapExpired bounds each lock hold and reports any remaining due buffers.
func (s *ringBufferShard) reapExpired(now int64, limit int) bool {
	for n := 0; n < limit && len(s.expires) > 0 && s.expires[0].idleDeadline <= now; n++ {
		s.expire(s.expires[0])
	}
	return len(s.expires) > 0 && s.expires[0].idleDeadline <= now
}
