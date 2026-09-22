package taskheap

import (
	"testing"
	"time"
)

func TestIndependentTasksWithSameRemoteID(t *testing.T) {
	m := NewManager()
	now := time.Now()
	first := &TaskItem{ContextId: "one", TaskId: "same", ToolCallId: "a", Exp: now}
	second := &TaskItem{ContextId: "two", TaskId: "same", ToolCallId: "b", Exp: now.Add(time.Hour)}
	m.Add(first)
	m.Add(second)
	if m.Len() != 2 {
		t.Fatal("remote task ID collision lost a task")
	}
	if _, ok := m.Delete("same"); ok {
		t.Fatal("ambiguous delete removed a task")
	}
	first.Exp = now.Add(time.Minute)
	m.Add(first)
	if len(m.PopExpired(now)) != 0 {
		t.Fatal("requeue did not update expiry")
	}
	snapshot := m.PeekAll()
	snapshot[0].TaskId = "mutated"
	snapshot[0].Exp = now.Add(time.Hour)
	ready := m.PopExpired(now.Add(time.Minute))
	if len(ready) != 1 || ready[0].ToolCallId != "a" || ready[0].TaskId != "same" {
		t.Fatal("snapshot corrupted task heap")
	}
	if item, ok := m.Delete("same"); !ok || item.ToolCallId != "b" || m.Len() != 0 {
		t.Fatal("remaining task cannot be deleted")
	}
}
