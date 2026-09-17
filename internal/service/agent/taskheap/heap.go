package taskheap

import "time"

type TaskItem struct {
	ContextId string
	TaskId    string

	ToolCallId string
	ToolName   string

	Exp   time.Time
	index int
}

type TaskHeap []*TaskItem

func (h TaskHeap) Len() int { return len(h) }

func (h TaskHeap) Less(i, j int) bool { return h[i].Exp.Before(h[j].Exp) }

func (h TaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Push appends an item; use container/heap.Push to maintain heap order.
func (h *TaskHeap) Push(value any) {
	item := value.(*TaskItem)
	item.index = len(*h)
	*h = append(*h, item)
}

// Pop removes the last item; use container/heap.Pop to remove the minimum.
func (h *TaskHeap) Pop() any {
	items := *h
	last := len(items) - 1
	item := items[last]
	items[last] = nil // Do not retain removed tasks in the backing array.
	item.index = -1
	*h = items[:last]
	return item
}
