package ringbuffer

// RingBuffer is a bounded FIFO, accessed only through manager-issued leases.
// Slots grow lazily up to the manager's capacity. A full buffer overwrites its
// oldest chunk; reading removes a chunk and releases its slot's byte slice.
type RingBuffer struct {
	shard     *ringBufferShard
	id        string
	chunks    []SSEChunk
	head      int
	size      int
	sequence  uint64
	producers int
	consumer  *ringBufferLease
	idle      ringBufferLease
}

func (b *RingBuffer) push(data []byte, capacity int) {
	if b.size == len(b.chunks) && b.size < capacity {
		n := min(capacity, max(1, len(b.chunks)*2))
		chunks := make([]SSEChunk, n)
		if b.size > 0 {
			first := copy(chunks, b.chunks[b.head:])
			copy(chunks[first:], b.chunks[:b.head])
		}
		b.chunks, b.head = chunks, 0
	}
	b.sequence++
	pos := (b.head + b.size) % len(b.chunks)
	b.chunks[pos] = SSEChunk{Sequence: b.sequence, Data: data}
	if b.size == len(b.chunks) {
		b.head = (b.head + 1) % len(b.chunks)
	} else {
		b.size++
	}
}
