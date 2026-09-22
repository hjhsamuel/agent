package a2a

import (
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/hjhsamuel/agent/pkg/tool"
)

func TestTextAndImmediateTaskCompletion(t *testing.T) {
	if text := FormatMessage(a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("hello"))); !strings.Contains(text, "hello") {
		t.Fatal("text lost")
	}
	result := taskResult(&a2a.Task{ID: "task", ContextID: "context", Status: a2a.TaskStatus{State: a2a.TaskStateCompleted}, Artifacts: []*a2a.Artifact{{Parts: a2a.ContentParts{a2a.NewTextPart("result")}}}})
	if result.Status != tool.TaskCompleted || !strings.Contains(result.Content, "result") {
		t.Fatalf("completion lost: %+v", result)
	}
}
