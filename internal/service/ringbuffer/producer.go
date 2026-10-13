package ringbuffer

// RingBufferProducer is a renewable one-minute lease. Push renews it; call Renew
// during longer pauses and Close when production ends. Multiple producers may
// share a buffer. All writes go through RingBufferManager.Push. Do not copy it.
type RingBufferProducer struct {
	manager *RingBufferManager
	lease   ringBufferLease
}

func (p *RingBufferProducer) Renew() error {
	if p == nil {
		return ErrRingBufferInvalidLease
	}
	return p.manager.renew(&p.lease)
}

// Close releases the producer lease. Repeated calls are harmless.
func (p *RingBufferProducer) Close() error {
	if p == nil {
		return ErrRingBufferInvalidLease
	}
	return p.manager.release(&p.lease)
}
