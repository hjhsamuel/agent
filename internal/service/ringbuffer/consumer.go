package ringbuffer

import "context"

// RingBufferConsumer is the exclusive renewable consumer lease. Next and Renew
// extend it. During a wait exceeding one minute, call Renew separately.
// Close immediately permits a replacement consumer. Do not copy it.
type RingBufferConsumer struct {
	manager      *RingBufferManager
	lease        ringBufferLease
	sequence     uint64 // Cursor fields are protected by lease.reading.
	reconnecting bool
	awaitingEnd  bool
}

// Next waits for the next chunk at this consumer's position, cancellation or
// lease termination. The first read applies AcquireConsumer's reconnect policy.
// Only one Next may be pending per consumer. Canceling ctx ends this read without
// closing the lease. Reads racing with Close may deliver an already claimed chunk.
func (c *RingBufferConsumer) Next(ctx context.Context) (SSEChunk, error) {
	if c == nil || c.manager == nil || c.lease.buffer == nil {
		return SSEChunk{}, ErrRingBufferInvalidLease
	}
	m, lease := c.manager, &c.lease
	if !lease.reading.CompareAndSwap(false, true) {
		if err := m.checkLease(lease); err != nil {
			return SSEChunk{}, err
		}
		return SSEChunk{}, ErrRingBufferReadBusy
	}
	defer lease.reading.Store(false)
	if err := m.renew(lease); err != nil {
		return SSEChunk{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return SSEChunk{}, err
		}
		var chunk SSEChunk
		var ok bool
		if c.reconnecting {
			// Producers can overwrite the client's sequence after acquisition
			// or while Next waits, so resolve until the first successful read.
			if c.awaitingEnd {
				chunk, ok = lease.buffer.latestEnd()
			} else {
				chunk, ok, c.awaitingEnd = lease.buffer.reconnect(c.sequence)
			}
		} else {
			chunk, ok = lease.buffer.next(c.sequence)
		}
		if ok {
			c.sequence = chunk.Sequence
			c.reconnecting, c.awaitingEnd = false, false
			return chunk, nil
		}
		select {
		case <-ctx.Done():
		case <-lease.done:
		case <-m.stop:
		case <-lease.wake:
		}
		// Check and renew after waiting, keeping the immediately-readable path
		// to one clock read and one lease CAS.
		if err := m.renew(lease); err != nil {
			return SSEChunk{}, err
		}
	}
}

func (c *RingBufferConsumer) Renew() error {
	if c == nil {
		return ErrRingBufferInvalidLease
	}
	return c.manager.renew(&c.lease)
}

// Close releases the consumer lease and wakes a pending Next.
func (c *RingBufferConsumer) Close() error {
	if c == nil {
		return ErrRingBufferInvalidLease
	}
	return c.manager.release(&c.lease)
}
