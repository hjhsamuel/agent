package ringbuffer

import (
	"encoding/json"
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

type SSEChunkType string

const (
	SSEChunkTypeNew            SSEChunkType = "new"
	SSEChunkTypeData           SSEChunkType = "content"
	SSEChunkTypeReasoning      SSEChunkType = "reasoning"
	SSEChunkTypeEnd            SSEChunkType = "end"  // Ends a complete logical chunk; Data must be empty.
	SSEChunkTypeHumanInTheLoop SSEChunkType = "hitl" // Requests human intervention; Data may carry details.
)

// SSEChunk holds opaque SSE bytes. Sequence starts at 1 per buffer. Reads return
// independent values sharing immutable Data strings with retained chunks.
// Overwriting old chunks can create gaps in delivered sequences.
type SSEChunk struct {
	Sequence uint64
	Type     SSEChunkType
	Data     string
}

type HITLEvent struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func (e HITLEvent) ToData() string {
	content, _ := json.Marshal(e)
	return string(content)
}

type ringBufferLease struct {
	buffer *RingBuffer
	kind   ringBufferLeaseKind
	// Active leases form a list for manager shutdown only, outside the expiry
	// heap. Links and stopCancel are protected by the shard lock.
	prev, next *ringBufferLease
	// Zero means connected; terminal states cannot be renewed.
	state      atomic.Uint64
	canceled   <-chan struct{}
	stopCancel func() bool   // Protected by the shard lock.
	wake       chan struct{} // Coalesced consumer notifications; never closed.
	done       chan struct{}
	reading    atomic.Bool
	waiting    atomic.Bool // Producers only notify a consumer preparing to park.
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
