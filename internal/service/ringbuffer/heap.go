package ringbuffer

// Only disconnected buffers are scheduled for expiry.
type ringBufferHeap []*RingBuffer

func (h ringBufferHeap) Len() int           { return len(h) }
func (h ringBufferHeap) Less(i, j int) bool { return h[i].idleDeadline < h[j].idleDeadline }
func (h ringBufferHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].idleIndex, h[j].idleIndex = i, j
}
func (h *ringBufferHeap) Push(v any) {
	b := v.(*RingBuffer)
	b.idleIndex = len(*h)
	*h = append(*h, b)
}
func (h *ringBufferHeap) Pop() any {
	old := *h
	n := len(old) - 1
	b := old[n]
	old[n] = nil
	b.idleIndex = -1
	*h = old[:n]
	return b
}
