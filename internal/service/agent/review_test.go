package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMainAgentRejectsConcurrentStarts(t *testing.T) {
	release := make(chan struct{})
	a := testAgent(t, &memoryStore{}, modelStub{stream: func([]*provider.Message) (*provider.Message, error) {
		<-release
		return &provider.Message{Role: provider.RoleAssistant, Content: "done"}, nil
	}})
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- a.Start("hello") }()
	}
	wg.Wait()
	close(results)
	started := 0
	for err := range results {
		if err == nil {
			started++
		}
	}
	close(release)
	a.Wait()
	if started != 1 {
		t.Fatalf("started %d runners", started)
	}
	if err := a.Start(""); err == nil {
		t.Fatal("finished runner restarted")
	}
}

func TestSubAgentFailureWhileWaitingForInput(t *testing.T) {
	for _, status := range []schema.RemoteTaskStatus{schema.RemoteTaskWaitingInput, schema.RemoteTaskInputted} {
		t.Run(string(status), func(t *testing.T) {
			store := newSubStore()
			a := subMain(t, store, nil)
			id, _ := store.CreateSubAgentTask(a.id, bson.NewObjectID(), "call", "work")
			store.remote[0].Status = status
			_ = store.FinishSubAgentTask(a.id, id, tool.TaskFailed, "child failed")
			if err := a.toolCheck(&taskheap.TaskItem{TaskId: id.Hex()}); err != nil {
				t.Fatal(err)
			}
			if len(store.finished) != 1 || store.finished[0].Content != "child failed" {
				t.Fatal("parent remained stuck waiting for input")
			}
		})
	}
}

func TestSubAgentRejectsMalformedInputEnvelope(t *testing.T) {
	store := newSubStore()
	parent, id := bson.NewObjectID(), bson.NewObjectID()
	store.states[id.Hex()] = &schema.TaskStoreServer{Parent: parent, Status: tool.TaskSubmitted}
	store.remote = []*schema.RemoteTaskStore{{Conversation: id, ContextId: "ctx", TaskId: "task", Status: schema.RemoteTaskWaitingInput}}
	a := &Agent{id: id, parent: parent, sub: &subRuntime{}, base: &BaseConfig{Store: store}}
	for _, input := range []string{`{"inputs":[]}`, `{"inputs":null}`, `{"inputs":"wrong"}`, `{"inputs":[{"context_id":"ctx","task_id":"task","content":42}]}`} {
		if err := a.Resume(input); err == nil {
			t.Errorf("forwarded malformed envelope as an answer: %s", input)
		}
	}
	if store.remote[0].Status != schema.RemoteTaskWaitingInput {
		t.Fatal("invalid input changed task state")
	}
}

type checkTool struct {
	echoTool
	result *tool.ToolResult
}

func (t checkTool) Check(context.Context, string, string, string) (*tool.ToolResult, error) {
	return t.result, nil
}

func TestToolPollingRejectsUnknownAndMissingTasks(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, nil)
	a.toolMap["echo"] = checkTool{result: &tool.ToolResult{Status: "unexpected"}}
	store.remote = []*schema.RemoteTaskStore{{Conversation: a.id, TaskId: "task", Status: schema.RemoteTaskSubmitted, Tool: &schema.TaskTool{Name: "echo", ToolCallId: "call"}}}
	if err := a.toolCheck(&taskheap.TaskItem{TaskId: "task"}); err == nil {
		t.Fatal("unknown status silently dropped pending task")
	}
	if err := a.toolCheck(&taskheap.TaskItem{TaskId: "missing"}); err == nil {
		t.Fatal("missing task silently dropped")
	}
}

func TestToolPollingScopesRepeatedRemoteTaskIDs(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, nil)
	store.remote = []*schema.RemoteTaskStore{
		{Conversation: a.id, ContextId: "one", TaskId: "shared", Status: schema.RemoteTaskDone, Content: "first", Tool: &schema.TaskTool{Name: "echo", ToolCallId: "first"}},
		{Conversation: a.id, ContextId: "two", TaskId: "shared", Status: schema.RemoteTaskDone, Content: "second", Tool: &schema.TaskTool{Name: "echo", ToolCallId: "second"}},
	}
	if err := a.toolCheck(&taskheap.TaskItem{ContextId: "two", TaskId: "shared", ToolName: "echo", ToolCallId: "second"}); err != nil {
		t.Fatal(err)
	}
	if len(store.finished) != 1 || store.finished[0].ToolCallId != "second" {
		t.Fatal("poll processed another tool instance sharing the same remote task ID")
	}
}

func TestSubAgentResumePersistenceFailureIsNotRetriedForever(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, nil)
	id := bson.NewObjectID()
	want := errors.New("write failed")
	a.runtime.main.children[id] = &Agent{id: id, parent: a.id, sub: &subRuntime{finishErr: want}}
	if err := a.toolResume(id.Hex(), id.Hex(), subAgentToolName, "call", "answer"); !errors.Is(err, want) {
		t.Fatalf("persistence error hidden: %v", err)
	}
	delete(a.runtime.main.children, id)
}

type cancelAfterCreateStore struct {
	*subStore
	cancel context.CancelFunc
}

func (s cancelAfterCreateStore) CreateSubAgentTask(parent, message bson.ObjectID, callID, content string) (bson.ObjectID, error) {
	id, err := s.subStore.CreateSubAgentTask(parent, message, callID, content)
	s.cancel()
	return id, err
}

func TestSubAgentCancellationBetweenCommitAndAdmission(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, nil)
	a.base.Store = cancelAfterCreateStore{subStore: store, cancel: a.runtime.main.cancel}
	ctx := context.WithValue(context.Background(), subAgentCallKey{}, subAgentCall{message: bson.NewObjectID(), id: "call"})
	if _, err := a.toolMap[subAgentToolName].Execute(ctx, "", `{"task":"work"}`); err == nil {
		t.Fatal("canceled main admitted a child")
	}
	if len(store.states) != 1 {
		t.Fatal("test did not commit a child task")
	}
	for _, state := range store.states {
		if state.Status != tool.TaskCanceled {
			t.Fatal("cancellation left a submitted orphan")
		}
	}
}
