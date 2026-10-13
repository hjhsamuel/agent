package ringbuffer

// RingBufferProducer stays connected until Close or its connection context is
// canceled. The manager renews its lease even without Push. No per-producer
// goroutine or timer is created. Do not copy it.
type RingBufferProducer struct {
	manager *RingBufferManager
	lease   ringBufferLease
}

// Renew explicitly extends the lease; connected producers also renew automatically.
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
