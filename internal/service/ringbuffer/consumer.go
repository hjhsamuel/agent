package ringbuffer

import "context"

// RingBufferConsumer stays exclusively connected until Close or its connection
// context is canceled, including during arbitrarily long waits for data.
// Close immediately permits a replacement consumer. Do not copy it.
type RingBufferConsumer struct {
	manager      *RingBufferManager
	lease        ringBufferLease
	sequence     uint64 // Cursor fields are protected by lease.reading.
	reconnecting bool
	awaitingEnd  bool
}

// Next returns all currently readable chunks at this consumer's position, waiting
// for data, cancellation or lease termination when none are available. Reads are
// level-triggered: buffered data is returned without waiting for a notification.
// The first read applies AcquireConsumer's reconnect policy.
// Only one Next may be pending per consumer. Canceling ctx ends this read without
// closing the lease. Reads racing with Close may deliver already claimed chunks.
func (c *RingBufferConsumer) Next(ctx context.Context) ([]SSEChunk, error) {
	if c == nil || c.manager == nil || c.lease.buffer == nil {
		return nil, ErrRingBufferInvalidLease
	}
	m, lease := c.manager, &c.lease
	if !lease.reading.CompareAndSwap(false, true) {
		if err := m.checkLease(lease); err != nil {
			return nil, err
		}
		return nil, ErrRingBufferReadBusy
	}
	defer lease.reading.Store(false)
	for {
		if err := m.checkLease(lease); err != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if chunks := c.readAvailable(); len(chunks) != 0 {
			return chunks, nil
		}
		// Arm before rechecking readiness: earlier publications are found by
		// the second read, later ones queue a wake. Writers need not notify a
		// consumer that is already draining data.
		lease.waiting.Store(true)
		if chunks := c.readAvailable(); len(chunks) != 0 {
			lease.waiting.Store(false)
			return chunks, nil
		}
		select {
		case <-ctx.Done():
		case <-lease.canceled:
		case <-lease.done:
		case <-m.stop:
		case <-lease.wake:
		}
		lease.waiting.Store(false)
	}
}

// readAvailable applies the reconnect policy until the first successful batch.
// Cursor fields are only accessed by the reader holding lease.reading.
func (c *RingBufferConsumer) readAvailable() []SSEChunk {
	var chunk SSEChunk
	var ok bool
	if c.reconnecting {
		if c.awaitingEnd {
			chunk, ok = c.lease.buffer.latestEnd()
		} else {
			chunk, ok, c.awaitingEnd = c.lease.buffer.reconnect(c.sequence)
		}
	} else {
		chunk, ok = c.lease.buffer.next(c.sequence)
	}
	if !ok {
		return nil
	}
	chunks := c.lease.buffer.available(chunk)
	c.sequence = chunks[len(chunks)-1].Sequence
	c.reconnecting, c.awaitingEnd = false, false
	return chunks
}

// Renew checks connection validity. Connections no longer need renewal; this
// method is retained for compatibility with existing callers.
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
