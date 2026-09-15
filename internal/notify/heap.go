package notify

import (
	"time"

	"github.com/hjhsamuel/agent/pkg/ringbuffer"
)

type bufferItem[T any] struct {
	id     string
	active time.Time
	buffer *ringbuffer.RingBuffer[T]

	index int
}

type bufferHeap[T any] []*bufferItem[T]

func (h bufferHeap[T]) Len() int {
	return len(h)
}

func (h bufferHeap[T]) Less(i, j int) bool {
	return h[i].active.Before(h[j].active)
}

func (h bufferHeap[T]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]

	h[i].index = i
	h[j].index = j
}

func (h *bufferHeap[T]) Push(x any) {
	item := x.(*bufferItem[T])
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *bufferHeap[T]) Pop() any {
	old := *h
	n := len(old)

	item := old[n-1]

	item.index = -1

	old[n-1] = nil
	*h = old[:n-1]

	return item
}
