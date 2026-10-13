package ringbuffer

type ringBufferLeaseHeap []*ringBufferLease

func (h ringBufferLeaseHeap) Len() int           { return len(h) }
func (h ringBufferLeaseHeap) Less(i, j int) bool { return h[i].deadline < h[j].deadline }
func (h ringBufferLeaseHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}
func (h *ringBufferLeaseHeap) Push(v any) {
	lease := v.(*ringBufferLease)
	lease.index = len(*h)
	*h = append(*h, lease)
}
func (h *ringBufferLeaseHeap) Pop() any {
	old := *h
	n := len(old) - 1
	lease := old[n]
	old[n] = nil
	lease.index = -1
	*h = old[:n]
	return lease
}
