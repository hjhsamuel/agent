package taskheap

import (
	"container/heap"
	"time"
)

type Manager struct {
	buffer map[taskKey]*TaskItem
	heap   TaskHeap
}

type taskKey struct{ context, task, call, tool string }

func key(item *TaskItem) taskKey {
	return taskKey{item.ContextId, item.TaskId, item.ToolCallId, item.ToolName}
}

func (m *Manager) Add(item *TaskItem) {
	if item == nil {
		return
	}

	if existing, ok := m.buffer[key(item)]; ok {
		index := existing.index
		*existing = *item
		existing.index = index
		heap.Fix(&m.heap, index)
		return
	}

	if m.buffer == nil {
		m.buffer = make(map[taskKey]*TaskItem)
	}
	owned := *item
	heap.Push(&m.heap, &owned)
	m.buffer[key(&owned)] = &owned
}

func (m *Manager) Delete(taskId string) (*TaskItem, bool) {
	// A bare task ID is only sufficient when it identifies exactly one task.
	var item *TaskItem
	for _, candidate := range m.buffer {
		if candidate.TaskId == taskId {
			if item != nil {
				return nil, false
			}
			item = candidate
		}
	}
	if item == nil {
		return nil, false
	}
	heap.Remove(&m.heap, item.index)
	delete(m.buffer, key(item))
	return item, true
}

func (m *Manager) PopExpired(now time.Time) []*TaskItem {
	var expired []*TaskItem
	for len(m.heap) > 0 && !m.heap[0].Exp.After(now) {
		item := heap.Pop(&m.heap).(*TaskItem)
		delete(m.buffer, key(item))
		expired = append(expired, item)
	}
	return expired
}

func (m *Manager) Len() int {
	return len(m.heap)
}

func (m *Manager) PeekAll() []*TaskItem {
	if len(m.heap) == 0 {
		return nil
	}
	dst := make([]*TaskItem, len(m.heap))
	for i, item := range m.heap {
		c := *item
		dst[i] = &c
	}
	return dst
}

func NewManager() *Manager {
	return &Manager{
		buffer: make(map[taskKey]*TaskItem),
		heap:   make(TaskHeap, 0),
	}
}
