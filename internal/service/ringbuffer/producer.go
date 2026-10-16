package ringbuffer

// RingBufferProducer stays connected until Close or its connection context is
// canceled. No per-producer goroutine, timer or periodic renewal is needed.
// Do not copy it.
type RingBufferProducer struct {
	manager *RingBufferManager
	lease   ringBufferLease
}

// Renew checks connection validity. Connections no longer need renewal; this
// method is retained for compatibility with existing callers.
func (p *RingBufferProducer) Renew() error {
	if p == nil {
		return ErrRingBufferInvalidLease
	}
	return p.manager.renew(&p.lease)
}

// Close marks the producer disconnected. Repeated calls are harmless.
func (p *RingBufferProducer) Close() error {
	if p == nil {
		return ErrRingBufferInvalidLease
	}
	return p.manager.release(&p.lease)
}
