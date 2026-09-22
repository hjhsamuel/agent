package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/entities"
	"github.com/hjhsamuel/agent/internal/llm"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type subStore struct {
	*memoryStore
	mu        sync.Mutex
	states    map[string]*schema.TaskStoreServer
	history   map[bson.ObjectID][]*schema.Message
	remote    []*schema.RemoteTaskStore
	finished  []*schema.FinishActiveReq
	finishErr error
}

func newSubStore() *subStore {
	return &subStore{memoryStore: &memoryStore{}, states: make(map[string]*schema.TaskStoreServer), history: make(map[bson.ObjectID][]*schema.Message)}
}

func (s *subStore) CreateSubAgentTask(parent, message bson.ObjectID, call, content string) (bson.ObjectID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pending := range s.remote {
		if pending.Conversation == parent && pending.Tool.ToolCallId == call {
			return bson.ObjectIDFromHex(pending.TaskId)
		}
	}
	id := bson.NewObjectID()
	s.states[id.Hex()] = &schema.TaskStoreServer{Parent: parent, ContextId: id.Hex(), TaskId: id.Hex(), Status: tool.TaskSubmitted}
	s.history[id] = []*schema.Message{{ID: bson.NewObjectID(), Conversation: id, Role: provider.RoleUser, Content: content}}
	s.remote = append(s.remote, &schema.RemoteTaskStore{Conversation: parent, Message: message, ContextId: id.Hex(), TaskId: id.Hex(), Status: schema.RemoteTaskSubmitted, Tool: &schema.TaskTool{Name: subAgentToolName, ToolCallId: call}})
	return id, nil
}

func (s *subStore) GetTaskStoreServer(filter bson.M) (*schema.TaskStoreServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[filter["task_id"].(string)]
	if state == nil || state.Parent != filter["parent"] {
		return nil, mongo.ErrNoDocuments
	}
	copy := *state
	return &copy, nil
}

func (s *subStore) FinishSubAgentTask(parent, id bson.ObjectID, status tool.TaskStatus, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finishErr != nil {
		return s.finishErr
	}
	state := s.states[id.Hex()]
	if state == nil || state.Parent != parent {
		return mongo.ErrNoDocuments
	}
	state.Status, state.Artifacts = status, []string{result}
	return nil
}

func (s *subStore) GetConversationMessages(filter bson.M, _ ...options.Lister[options.FindOptions]) ([]*schema.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*schema.Message(nil), s.history[filter["conversation"].(bson.ObjectID)]...), nil
}

func (s *subStore) AddConversationMessage(messages ...*schema.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range messages {
		s.history[m.Conversation] = append(s.history[m.Conversation], m)
	}
	return nil
}

func (s *subStore) ListRemoteTaskStores(filter bson.M) ([]*schema.RemoteTaskStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*schema.RemoteTaskStore
	for _, r := range s.remote {
		if r.Conversation != filter["conversation"] {
			continue
		}
		if status, ok := filter["status"]; ok && status != r.Status {
			continue
		}
		copy := *r
		out = append(out, &copy)
	}
	return out, nil
}

func (s *subStore) UpdateRemoteTaskStore(filter bson.M, update bson.M) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.remote {
		if r.Conversation == filter["conversation"] && r.ContextId == filter["context_id"] && r.TaskId == filter["task_id"] {
			if status, ok := filter["status"]; ok && status != r.Status {
				continue
			}
			fields := update["$set"].(bson.M)
			if status, ok := fields["status"].(schema.RemoteTaskStatus); ok {
				r.Status = status
			}
			if content, ok := fields["content"].(string); ok {
				r.Content = content
			}
			return nil
		}
	}
	return mongo.ErrNoDocuments
}

func (s *subStore) ActiveTaskFinished(_ bson.ObjectID, requests ...*schema.FinishActiveReq) (bson.ObjectID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = append(s.finished, requests...)
	return bson.NewObjectID(), nil
}

func (s *subStore) ResumeSubAgentTasks(conversation bson.ObjectID, inputs []*schema.RemoteTaskStore) error {
	for _, input := range inputs {
		if err := s.UpdateRemoteTaskStore(bson.M{"conversation": conversation, "context_id": input.ContextId, "task_id": input.TaskId, "status": schema.RemoteTaskWaitingInput}, bson.M{"$set": bson.M{"status": schema.RemoteTaskInputted, "content": input.Content}}); err != nil {
			return err
		}
	}
	return nil
}

type subModel struct {
	modelStub
	run func(context.Context, []*provider.Message, *provider.ChatConfig, provider.YieldFunc) (*provider.Message, error)
}

func (m subModel) Stream(ctx context.Context, _, _ string, messages []*provider.Message, cfg *provider.ChatConfig, yield provider.YieldFunc) (*provider.Message, error) {
	return m.run(ctx, messages, cfg, yield)
}

func subMain(t *testing.T, store *subStore, run func(context.Context, []*provider.Message, *provider.ChatConfig, provider.YieldFunc) (*provider.Message, error)) *Agent {
	t.Helper()
	m := llm.New("test", nil, subModel{run: run})
	a := NewMainAgent(context.Background(), bson.NewObjectID(), &BaseConfig{User: &entities.UserInfo{ID: 1}, Store: store, Provider: m, Compact: m, Tools: []tool.Tool{echoTool{}}, Up: make(chan *notify.UpperEvent, 32)}).(*Agent)
	t.Cleanup(func() { _ = a.closeChildren() })
	return a
}

func TestSubAgentToolResultAndIsolation(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, func(ctx context.Context, messages []*provider.Message, cfg *provider.ChatConfig, yield provider.YieldFunc) (*provider.Message, error) {
		if len(messages) != 1 || messages[0].Content != "delegated context" {
			t.Error("child did not receive isolated task history")
		}
		for _, definition := range cfg.Tool {
			if definition.GetFunction().Name == subAgentToolName {
				t.Error("child can recursively delegate")
			}
		}
		if err := yield(&provider.StreamChunk{Type: provider.Completion, Content: "private"}, nil); err != nil {
			return nil, err
		}
		return &provider.Message{Role: provider.RoleAssistant, Content: "child result"}, nil
	})
	a.runtime.OldMessages = []*provider.Message{{Role: provider.RoleUser, Content: "private parent history"}}
	if err := a.toolExecute(bson.NewObjectID(), "call", subAgentToolName, `{"task":"delegated context"}`); err != nil {
		t.Fatal(err)
	}
	if len(a.runtime.main.children) != 1 || a.runtime.tasks.Len() != 1 {
		t.Fatal("child not owned and queued by main")
	}
	var id bson.ObjectID
	for key, child := range a.runtime.main.children {
		id = key
		child.Wait()
	}
	if len(a.base.Up) != 0 {
		t.Fatal("subagent leaked events to service")
	}
	if err := a.toolCheck(&taskheap.TaskItem{TaskId: id.Hex()}); err != nil {
		t.Fatal(err)
	}
	if len(store.finished) != 1 || store.finished[0].ToolCallId != "call" || store.finished[0].Content != "child result" {
		t.Fatalf("wrong paired result: %+v", store.finished)
	}
	other := subMain(t, store, nil)
	if _, err := other.toolMap[subAgentToolName].Check(context.Background(), "", id.Hex(), id.Hex()); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("foreign main accessed child: %v", err)
	}
	if _, err := a.toolMap[subAgentToolName].Check(context.Background(), "", "wrong", id.Hex()); err == nil {
		t.Fatal("accepted mismatched context")
	}
}

func TestSubAgentEndToEndMainLoop(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, func(_ context.Context, messages []*provider.Message, _ *provider.ChatConfig, _ provider.YieldFunc) (*provider.Message, error) {
		if messages[0].Content == "child work" {
			if len(messages) == 1 {
				return &provider.Message{Role: provider.RoleAssistant, ToolCalls: []*provider.ToolCall{{ID: "nested", Name: "echo", Arguments: "{}"}}}, nil
			}
			if len(messages) != 3 || messages[2].ToolCallId != "nested" {
				t.Error("child tool result not paired")
			}
			return &provider.Message{Role: provider.RoleAssistant, Content: "delegated result"}, nil
		}
		if len(messages) == 1 {
			return &provider.Message{Role: provider.RoleAssistant, ToolCalls: []*provider.ToolCall{{ID: "delegate", Name: subAgentToolName, Arguments: `{"task":"child work"}`}}}, nil
		}
		if len(messages) != 3 || messages[2].Role != provider.RoleTool || messages[2].ToolCallId != "delegate" || messages[2].Content != "delegated result" {
			t.Errorf("main received incorrect child result: %+v", messages)
		}
		return &provider.Message{Role: provider.RoleAssistant, Content: "main answer"}, nil
	})
	store.history[a.id] = []*schema.Message{{Conversation: a.id, Role: provider.RoleUser, Content: "parent work"}}
	if err := a.Start("parent work"); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { a.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("main did not consume child result")
	}
	if len(a.runtime.main.children) != 0 {
		t.Fatal("main completed before releasing children")
	}
	for len(a.base.Up) != 0 {
		event := <-a.base.Up
		if event.ID != a.id.Hex() || event.Error != nil {
			t.Fatalf("invalid service event: %+v", event)
		}
	}
	if messages := store.history[a.id]; messages[len(messages)-1].Content != "main answer" {
		t.Fatal("main answer not persisted")
	}
}

func TestSubAgentLifetimeFollowsMainNotToolCall(t *testing.T) {
	store := newSubStore()
	started := make(chan struct{}, 2)
	a := subMain(t, store, func(ctx context.Context, _ []*provider.Message, _ *provider.ChatConfig, _ provider.YieldFunc) (*provider.Message, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	for _, call := range []string{"one", "two"} {
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), subAgentCallKey{}, subAgentCall{message: bson.NewObjectID(), id: call}))
		if _, err := a.toolMap[subAgentToolName].Execute(ctx, "", `{"task":"work"}`); err != nil {
			t.Fatal(err)
		}
		cancel()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("children did not start concurrently")
		}
	}
	for _, child := range a.runtime.main.children {
		if child.ctx.Err() != nil {
			t.Fatal("tool request cancellation killed child")
		}
	}
	done := make(chan struct{})
	go func() { a.loopDefer(nil); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("main failed to join canceled children")
	}
	if event := <-a.base.Up; !event.Finished || event.ID != a.id.Hex() {
		t.Fatalf("wrong parent completion: %+v", event)
	}
	for _, state := range store.states {
		if state.Status != tool.TaskCanceled {
			t.Fatalf("child not canceled before main completion: %+v", state)
		}
	}
	if len(a.runtime.main.children) != 0 {
		t.Fatal("children retained after completion")
	}
	ctx := context.WithValue(context.Background(), subAgentCallKey{}, subAgentCall{id: "late"})
	if _, err := a.toolMap[subAgentToolName].Execute(ctx, "", `{"task":"late"}`); err == nil {
		t.Fatal("closed main accepted work")
	}
	if len(store.states) != 2 {
		t.Fatal("closed main persisted orphan task")
	}
}

func TestSubAgentRecoveryDoesNotRepeatFinalResponse(t *testing.T) {
	store := newSubStore()
	a := subMain(t, store, func(context.Context, []*provider.Message, *provider.ChatConfig, provider.YieldFunc) (*provider.Message, error) {
		t.Error("completed model call replayed")
		return nil, errors.New("unexpected model call")
	})
	id, _ := store.CreateSubAgentTask(a.id, bson.NewObjectID(), "call", "task")
	_ = store.AddConversationMessage(&schema.Message{Conversation: id, Role: provider.RoleAssistant, Content: "saved result"})
	child, err := a.child(id)
	if err != nil {
		t.Fatal(err)
	}
	child.Wait()
	result, err := a.toolMap[subAgentToolName].Check(context.Background(), "", id.Hex(), id.Hex())
	if err != nil || result.Status != tool.TaskCompleted || result.Content != "saved result" {
		t.Fatalf("bad recovered result: %+v, %v", result, err)
	}
	if _, err := a.child(id); err != nil || len(a.runtime.main.children) != 1 {
		t.Fatal("recovery duplicated child")
	}
}

func TestSubAgentInputTargetsStayWithinChild(t *testing.T) {
	store := newSubStore()
	parent, id := bson.NewObjectID(), bson.NewObjectID()
	store.states[id.Hex()] = &schema.TaskStoreServer{Parent: parent, TaskId: id.Hex(), Status: tool.TaskSubmitted}
	store.remote = []*schema.RemoteTaskStore{{Conversation: id, ContextId: "ctx", TaskId: "pending", Status: schema.RemoteTaskWaitingInput, Content: "Which order?"}}
	child := &Agent{id: id, parent: parent, sub: &subRuntime{}, base: &BaseConfig{Store: store}}
	state, err := child.GetState()
	if err != nil || state.Status != tool.TaskInputRequired {
		t.Fatalf("missing input request: %+v, %v", state, err)
	}
	if err := child.Resume(`{"inputs":[{"context_id":"other","task_id":"pending","content":"42"}]}`); err == nil {
		t.Fatal("accepted another task's input")
	}
	if err := child.Resume(`{"inputs":[{"context_id":"ctx","task_id":"pending","content":"42"}]}`); err != nil {
		t.Fatal(err)
	}
	if store.remote[0].Status != schema.RemoteTaskInputted || store.remote[0].Content != "42" {
		t.Fatal("input not routed to nested tool")
	}
	if state, err := child.GetState(); err != nil || state.Status != tool.TaskWorking {
		t.Fatal("child did not return to working")
	}
}

func TestSubAgentFailureAndPersistenceError(t *testing.T) {
	for _, persistErr := range []error{nil, errors.New("write failed")} {
		t.Run("persist="+formatError(persistErr), func(t *testing.T) {
			store := newSubStore()
			store.finishErr = persistErr
			a := subMain(t, store, func(context.Context, []*provider.Message, *provider.ChatConfig, provider.YieldFunc) (*provider.Message, error) {
				return nil, errors.New("model failed")
			})
			if err := a.toolExecute(bson.NewObjectID(), "call", subAgentToolName, `{"task":"work"}`); err != nil {
				t.Fatal(err)
			}
			for _, child := range a.runtime.main.children {
				child.Wait()
				state, err := child.GetState()
				if persistErr != nil {
					if !errors.Is(err, persistErr) {
						t.Fatalf("persistence error lost: %v", err)
					}
					continue
				}
				if err != nil || state.Status != tool.TaskFailed || state.Artifacts[0] != "model failed" {
					t.Fatalf("failure not returned: %+v, %v", state, err)
				}
			}
		})
	}
}

func formatError(err error) string {
	if err == nil {
		return "nil"
	}
	return err.Error()
}

func TestSubAgentToolIsPerMainAndInvalidInputDoesNotCreateTask(t *testing.T) {
	store := newSubStore()
	base := &BaseConfig{Store: store, Tools: []tool.Tool{echoTool{}}}
	a := NewMainAgent(context.Background(), bson.NewObjectID(), base).(*Agent)
	b := NewMainAgent(context.Background(), bson.NewObjectID(), base).(*Agent)
	defer a.closeChildren()
	defer b.closeChildren()
	if len(base.Tools) != 1 || a.toolMap[subAgentToolName] == b.toolMap[subAgentToolName] {
		t.Fatal("main agents share tool ownership")
	}
	for _, arguments := range []string{`{`, `{}`, `{"task":" "}`} {
		result, err := a.toolMap[subAgentToolName].Execute(context.Background(), "", arguments)
		if err != nil || result.Status != tool.TaskFailed {
			t.Fatalf("invalid arguments not reported: %+v, %v", result, err)
		}
	}
	if len(store.states) != 0 {
		t.Fatal("invalid input created a child")
	}
}
