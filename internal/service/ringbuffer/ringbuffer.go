package ringbuffer

import "sync/atomic"

// RingBuffer is a bounded FIFO. Writers publish immutable chunks with CAS;
// readers claim sequence numbers with CAS. Full buffers overwrite old chunks.
// Storage is allocated in small segments as slots are first used.
// RingBuffer must not be copied.
type RingBuffer struct {
	shard       *ringBufferShard
	id          string
	storage     atomic.Pointer[ringBufferStorage]
	retired     atomic.Bool
	written     atomic.Uint64
	read        atomic.Uint64
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

func (b *RingBuffer) push(data []byte, capacity int) (uint64, bool) {
	storage := b.getStorage(capacity)
	if storage == nil {
		return 0, false
	}
	sequence := b.written.Add(1)
	chunk := &SSEChunk{Sequence: sequence, Data: data}
	b.publish(storage, chunk)
	return sequence, true
}

func (b *RingBuffer) publish(storage *ringBufferStorage, chunk *SSEChunk) {
	sequence := chunk.Sequence
	slot := storage.slot(sequence, true)
	for {
		// A delayed writer must not replace a newer chunk, or republish one
		// already consumed/dropped while it was waiting to run.
		if sequence <= b.read.Load() || b.written.Load()-sequence >= storage.capacity {
			return
		}
		old := slot.Load()
		if old != nil && old.Sequence >= sequence {
			return
		}
		if slot.CompareAndSwap(old, chunk) {
			if sequence <= b.read.Load() || b.written.Load()-sequence >= storage.capacity {
				slot.CompareAndSwap(chunk, nil)
			}
			return
		}
	}
}

func (b *RingBuffer) pop() (SSEChunk, bool) {
	storage := b.storage.Load()
	if storage == nil {
		return SSEChunk{}, false
	}
	for {
		read, written := b.read.Load(), b.written.Load()
		if read >= written {
			return SSEChunk{}, false
		}
		if written-read > storage.capacity {
			b.read.CompareAndSwap(read, written-storage.capacity)
			continue
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
		if b.read.CompareAndSwap(read, sequence) {
			slot.CompareAndSwap(chunk, nil) // Do not clear a newer writer's chunk.
			return *chunk, true
		}
	}
}

func (b *RingBuffer) clear() {
	b.retired.Store(true)
	b.storage.Store(nil) // In-flight operations own snapshots; stale handles retain no payloads.
}
