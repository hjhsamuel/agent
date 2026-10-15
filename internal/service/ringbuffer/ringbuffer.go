package ringbuffer

import (
	"strings"
	"sync/atomic"
)

// RingBuffer is a bounded FIFO. Writers publish immutable chunks with CAS;
// consumers keep independent read positions. Full buffers overwrite old chunks.
// Storage is allocated in small segments as slots are first used.
// RingBuffer must not be copied.
type RingBuffer struct {
	shard       *ringBufferShard
	id          string
	storage     atomic.Pointer[ringBufferStorage]
	retired     atomic.Bool
	written     atomic.Uint64
	consumer    atomic.Pointer[ringBufferLease]
	retainUntil atomic.Int64
	producers   int // Lifecycle fields are protected by the shard lock.
	idle        ringBufferLease
}

type ringBufferStorage struct {
	capacity uint64
	segments []atomic.Pointer[ringBufferSegment]
}
type ringBufferSegment struct {
	slots [ringBufferSegmentSize]atomic.Pointer[SSEChunk]
}

func (b *RingBuffer) getStorage(capacity int) *ringBufferStorage {
	if storage := b.storage.Load(); storage != nil {
		return storage
	}
	if b.retired.Load() {
		return nil
	}
	storage := &ringBufferStorage{
		capacity: uint64(capacity),
		segments: make([]atomic.Pointer[ringBufferSegment], (capacity-1)/ringBufferSegmentSize+1),
	}
	if !b.storage.CompareAndSwap(nil, storage) {
		return b.storage.Load()
	}
	// A concurrent retirement must not leave newly allocated storage attached.
	if b.retired.Load() {
		b.storage.CompareAndSwap(storage, nil)
		return nil
	}
	return storage
}

func (s *ringBufferStorage) slot(sequence uint64, allocate bool) *atomic.Pointer[SSEChunk] {
	pos := (sequence - 1) % s.capacity
	segment := s.segments[pos/ringBufferSegmentSize].Load()
	if segment == nil {
		if !allocate {
			return nil
		}
		segment = new(ringBufferSegment)
		if !s.segments[pos/ringBufferSegmentSize].CompareAndSwap(nil, segment) {
			segment = s.segments[pos/ringBufferSegmentSize].Load()
		}
	}
	return &segment.slots[pos%ringBufferSegmentSize]
}

func (b *RingBuffer) push(chunk SSEChunk, capacity int) (uint64, bool) {
	storage := b.getStorage(capacity)
	if storage == nil {
		return 0, false
	}
	sequence := b.written.Add(1)
	chunk.Sequence = sequence
	b.publish(storage, &chunk)
	return sequence, true
}

func (b *RingBuffer) publish(storage *ringBufferStorage, chunk *SSEChunk) {
	sequence := chunk.Sequence
	slot := storage.slot(sequence, true)
	for {
		// A delayed writer must not replace a newer chunk or publish a chunk
		// that is already outside the retained window.
		if b.written.Load()-sequence >= storage.capacity {
			return
		}
		old := slot.Load()
		if old != nil && old.Sequence >= sequence {
			return
		}
		if slot.CompareAndSwap(old, chunk) {
			if b.written.Load()-sequence >= storage.capacity {
				slot.CompareAndSwap(chunk, nil)
			}
			return
		}
	}
}

// reconnect resumes after a retained sequence, or returns the latest retained
// end marker itself when the sequence is missing. Without an end marker, it
// reports no data so Next waits for a safe boundary.
// The third result records that the sequence was missing, so waiting consumers
// keep waiting for a boundary even if a new buffer later reuses that sequence.
func (b *RingBuffer) reconnect(sequence uint64) (SSEChunk, bool, bool) {
	if sequence == 0 {
		chunk, ok := b.next(0)
		return chunk, ok, false
	}
	storage := b.storage.Load()
	if storage == nil {
		return SSEChunk{}, false, true
	}
	written := b.written.Load()
	if sequence <= written && written-sequence < storage.capacity {
		if slot := storage.slot(sequence, false); slot != nil {
			if chunk := slot.Load(); chunk != nil && chunk.Sequence == sequence {
				next, ok := b.next(sequence)
				return next, ok, false
			}
		}
	}
	chunk, ok := b.latestEnd()
	return chunk, ok, true
}

func (b *RingBuffer) latestEnd() (SSEChunk, bool) {
	storage := b.storage.Load()
	if storage == nil {
		return SSEChunk{}, false
	}
	written := b.written.Load()
	oldest := uint64(1)
	if written >= storage.capacity {
		oldest = written - storage.capacity + 1
	}
	for current := written; current >= oldest; current-- {
		if slot := storage.slot(current, false); slot != nil {
			if chunk := slot.Load(); chunk != nil && chunk.Sequence == current && chunk.Type == SSEChunkTypeEnd {
				// Return the immutable marker snapshot directly, so a concurrent
				// overwrite cannot substitute an ordinary data chunk at this position.
				return SSEChunk{Sequence: chunk.Sequence, Type: chunk.Type}, true
			}
		}
	}
	return SSEChunk{}, false
}

// next reads without removing the chunk so clients can replay retained data.
func (b *RingBuffer) next(after uint64) (SSEChunk, bool) {
	storage := b.storage.Load()
	if storage == nil {
		return SSEChunk{}, false
	}
	for {
		read, written := after, b.written.Load()
		if read >= written {
			return SSEChunk{}, false
		}
		if written-read > storage.capacity {
			read = written - storage.capacity
		}
		sequence := read + 1
		slot := storage.slot(sequence, false)
		if slot == nil {
			return SSEChunk{}, false
		}
		chunk := slot.Load()
		if chunk == nil || chunk.Sequence != sequence {
			// A ticket can precede publication. Its writer signals the consumer
			// when published; overflow may advance past the pending ticket.
			if b.written.Load()-read > storage.capacity {
				continue
			}
			return SSEChunk{}, false
		}
		return SSEChunk{Sequence: chunk.Sequence, Type: chunk.Type, Data: strings.Clone(chunk.Data)}, true
	}
}

func (b *RingBuffer) clear() {
	b.retired.Store(true)
	b.storage.Store(nil) // In-flight operations own snapshots; stale handles retain no payloads.
}
