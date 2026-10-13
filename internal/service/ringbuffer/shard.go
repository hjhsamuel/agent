package ringbuffer

import (
	"container/heap"
	"sync"
)

type ringBufferShard struct {
	mu      sync.Mutex
	buffers map[string]*RingBuffer
	expires ringBufferLeaseHeap
}

// endLease requires the shard lock and an already-inactive lease state.
func (s *ringBufferShard) endLease(lease *ringBufferLease, idleDeadline int64) {
	heap.Remove(&s.expires, lease.index)
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
	ringBufferExtend(&b.retainUntil, idleDeadline)
	if b.producers == 0 && b.consumer.Load() == nil {
		b.idle.deadline = b.retainUntil.Load()
		heap.Push(&s.expires, &b.idle)
	}
}

// expire requires the shard lock. Renewed leases are rescheduled lazily.
func (s *ringBufferShard) expire(lease *ringBufferLease, now int64) {
	if lease.kind == ringBufferIdleLease {
		if until := lease.buffer.retainUntil.Load(); until > lease.deadline {
			lease.deadline = until
			heap.Fix(&s.expires, lease.index)
		}
		if lease.deadline > now {
			return
		}
		heap.Remove(&s.expires, lease.index)
		b := lease.buffer
		delete(s.buffers, b.id)
		b.clear()
		if len(s.buffers) == 0 {
			s.buffers, s.expires = nil, nil
		}
		return
	}
	for {
		state := lease.state.Load()
		if ringBufferLeaseError(state) != nil {
			return
		}
		if lease.kind == ringBufferProducerLease {
			if lease.ctx != nil && lease.ctx.Err() != nil {
				lease.state.Store(ringBufferLeaseClosed)
				s.endLease(lease, now+int64(RingBufferLeaseTTL))
				return
			}
			// The connection is still live. A stopped scheduler or long Push pause
			// must not expire it; renew from the current monotonic time.
			if int64(state) <= now {
				deadline := now + int64(RingBufferLeaseTTL)
				if !lease.state.CompareAndSwap(state, uint64(deadline)) {
					continue
				}
				ringBufferExtend(&lease.buffer.retainUntil, deadline)
				state = uint64(deadline)
			}
		} else if int64(state) <= now {
			if !lease.state.CompareAndSwap(state, ringBufferLeaseExpired) {
				continue
			}
			s.endLease(lease, int64(state))
			return
		}
		lease.deadline = int64(state)
		heap.Fix(&s.expires, lease.index)
		return
	}
}

// reapExpired bounds each lock hold and reports any remaining due events.
func (s *ringBufferShard) reapExpired(now int64, limit int) bool {
	for n := 0; n < limit && len(s.expires) > 0 && s.expires[0].deadline <= now; n++ {
		s.expire(s.expires[0], now)
	}
	return len(s.expires) > 0 && s.expires[0].deadline <= now
}
