package taskheap

import (
	"container/heap"
	"time"
)

type Manager struct {
	buffer map[string]*TaskItem
	heap   TaskHeap
}

func (m *Manager) Add(item *TaskItem) {
	if item == nil {
		return
	}

	if existing, ok := m.buffer[item.TaskId]; ok {
		index := existing.index
		*existing = *item
		existing.index = index
		heap.Fix(&m.heap, index)
		return
	}

	if m.buffer == nil {
		m.buffer = make(map[string]*TaskItem)
	}
	owned := *item
	heap.Push(&m.heap, &owned)
	m.buffer[owned.TaskId] = &owned
}

func (m *Manager) Delete(taskId string) (*TaskItem, bool) {
	item, ok := m.buffer[taskId]
	if !ok {
		return nil, false
	}
	heap.Remove(&m.heap, item.index)
	delete(m.buffer, taskId)
	return item, true
}

func (m *Manager) PopExpired(now time.Time) []*TaskItem {
	var expired []*TaskItem
	for len(m.heap) > 0 && !m.heap[0].Exp.After(now) {
		item := heap.Pop(&m.heap).(*TaskItem)
		delete(m.buffer, item.TaskId)
		expired = append(expired, item)
	}
	return expired
}

func (m *Manager) Len() int {
	return len(m.heap)
}

func NewManager() *Manager {
	return &Manager{
		buffer: make(map[string]*TaskItem),
		heap:   make(TaskHeap, 0),
	}
}
