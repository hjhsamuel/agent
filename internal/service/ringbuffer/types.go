package ringbuffer

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

const (
	RingBufferLeaseTTL        = time.Minute
	DefaultRingBufferCapacity = 1024
	ringBufferShardCount      = 32
	ringBufferReapBatch       = 256
	ringBufferSegmentSize     = 32
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
	ErrRingBufferInvalidChunk  = errors.New("invalid SSE chunk type or payload")
)

type SSEChunkType uint8

const (
	SSEChunkTypeData SSEChunkType = iota
	SSEChunkTypeReasoning
	SSEChunkTypeEnd            // Ends a complete logical chunk; Data must be empty.
	SSEChunkTypeHumanInTheLoop // Requests human intervention; Data may carry details.
)

// SSEChunk holds opaque SSE bytes. Sequence starts at 1 per buffer. Each read
// owns a copy of Data, leaving retained chunks immutable for reconnection.
// Overwriting old chunks can create gaps in delivered sequences.
type SSEChunk struct {
	Sequence uint64
	Type     SSEChunkType
	Data     []byte
}

type ringBufferLease struct {
	buffer *RingBuffer
	kind   ringBufferLeaseKind
	// Heap metadata is protected by the shard lock; hot-path renewals do not
	// change it. The reaper checks the atomic state when this deadline arrives.
	deadline int64
	index    int
	// Active values are monotonic nanosecond deadlines relative to manager.start.
	// Inactive values have the high bit set. Renew and expiry CAS this same word
	// so a stale expiry decision cannot discard a completed renewal.
	state      atomic.Uint64
	ctx        context.Context
	stopCancel func() bool   // Protected by the shard lock.
	wake       chan struct{} // Coalesced consumer notifications; never closed.
	done       chan struct{}
	reading    atomic.Bool
}

const (
	ringBufferLeaseClosed uint64 = 1<<63 + iota
	ringBufferLeaseExpired
	ringBufferManagerClosed
)

func ringBufferLeaseError(state uint64) error {
	switch state {
	case ringBufferLeaseClosed:
		return ErrRingBufferLeaseClosed
	case ringBufferLeaseExpired:
		return ErrRingBufferLeaseExpired
	case ringBufferManagerClosed:
		return ErrRingBufferManagerClosed
	default:
		return nil
	}
}

func ringBufferExtend(deadline *atomic.Int64, value int64) {
	for old := deadline.Load(); old < value; old = deadline.Load() {
		if deadline.CompareAndSwap(old, value) {
			return
		}
	}
}
