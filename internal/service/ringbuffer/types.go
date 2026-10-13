package ringbuffer

import (
	"errors"
	"time"
)

const (
	RingBufferLeaseTTL        = time.Minute
	DefaultRingBufferCapacity = 1024
	ringBufferShardCount      = 32
	ringBufferReapBatch       = 256
)

type ringBufferLeaseKind uint8

const (
	ringBufferProducerLease ringBufferLeaseKind = iota
	ringBufferConsumerLease
	ringBufferIdleLease
)

var (
	ErrRingBufferManagerClosed = errors.New("ringbuffer manager is closed")
	ErrRingBufferConsumerBusy  = errors.New("ringbuffer already has a consumer")
	ErrRingBufferLeaseExpired  = errors.New("ringbuffer lease has expired")
	ErrRingBufferLeaseClosed   = errors.New("ringbuffer lease is closed")
	ErrRingBufferInvalidLease  = errors.New("invalid ringbuffer lease")
	ErrRingBufferReadBusy      = errors.New("consumer already has a pending read")
)

type ringBufferLease struct {
	buffer   *RingBuffer
	kind     ringBufferLeaseKind
	deadline time.Time
	index    int
	err      error
	wake     chan struct{} // Allocated only for consumers; notifications coalesce.
	reading  bool
}

// SSEChunk holds opaque SSE bytes. Sequence starts at 1 for each new buffer and
// increases even when older chunks are overwritten. The reader owns Data.
type SSEChunk struct {
	Sequence uint64
	Data     []byte
}
