package ringbuffer

import (
	"context"
	"time"
)

// RingBufferConsumer is the exclusive renewable consumer lease for a buffer.
// Next renews it on entry and after delivery. During a wait longer than one
// minute, call Renew separately to keep the lease alive. Close permits another
// consumer to acquire the remaining backlog immediately. Do not copy it.
type RingBufferConsumer struct {
	manager *RingBufferManager
	lease   ringBufferLease
}

// Next waits for the oldest retained chunk, cancellation, or lease termination.
// Only one Next call may be pending on a consumer. Context cancellation ends
// that read; it does not close the consumer lease. There is no producer EOF:
// a new producer may join the buffer until its idle lease expires.
func (c *RingBufferConsumer) Next(ctx context.Context) (SSEChunk, error) {
	if c == nil || c.manager == nil || c.lease.buffer == nil {
		return SSEChunk{}, ErrRingBufferInvalidLease
	}
	m, lease := c.manager, &c.lease
	s := lease.buffer.shard
	s.mu.Lock()
	if err := m.checkLease(lease, time.Now()); err != nil {
		s.mu.Unlock()
		return SSEChunk{}, err
	}
	if lease.reading {
		s.mu.Unlock()
		return SSEChunk{}, ErrRingBufferReadBusy
	}
	lease.reading = true
	m.renewLocked(lease, time.Now())
	defer func() {
		s.mu.Lock()
		lease.reading = false
		s.mu.Unlock()
	}()
	for {
		if err := m.checkLease(lease, time.Now()); err != nil {
			s.mu.Unlock()
			return SSEChunk{}, err
		}
		if err := ctx.Err(); err != nil {
			s.mu.Unlock()
			return SSEChunk{}, err
		}
		b := lease.buffer
		if b.size > 0 {
			chunk := b.chunks[b.head]
			b.chunks[b.head] = SSEChunk{}
			b.head = (b.head + 1) % len(b.chunks)
			b.size--
			m.renewLocked(lease, time.Now())
			s.mu.Unlock()
			return chunk, nil
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
		case <-lease.wake:
		}
		s.mu.Lock()
	}
}

func (c *RingBufferConsumer) Renew() error {
	if c == nil {
		return ErrRingBufferInvalidLease
	}
	return c.manager.renew(&c.lease)
}

// Close releases the consumer lease and wakes any pending Next call.
func (c *RingBufferConsumer) Close() error {
	if c == nil {
		return ErrRingBufferInvalidLease
	}
	return c.manager.release(&c.lease)
}
