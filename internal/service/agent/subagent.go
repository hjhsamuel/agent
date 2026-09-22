package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/service/agent/taskheap"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type subRuntime struct {
	mu        sync.Mutex
	started   bool
	finishErr error
}

// closeChildren seals admission before cancellation and joins every child.
// The main runner calls this before reporting its own completion to service.
func (a *Agent) closeChildren() error {
	m := a.runtime.main
	m.mu.Lock()
	m.closed = true
	m.cancel()
	children := make([]*Agent, 0, len(m.children))
	for _, child := range m.children {
		children = append(children, child)
	}
	m.mu.Unlock()
	var err error
	for _, child := range children {
		child.Wait()
		child.sub.mu.Lock()
		err = errors.Join(err, child.sub.finishErr)
		child.sub.mu.Unlock()
	}
	m.mu.Lock()
	clear(m.children)
	m.mu.Unlock()
	return err
}

func (a *Agent) child(id bson.ObjectID) (*Agent, error) {
	m := a.runtime.main
	if m == nil {
		return nil, errors.New("only main agents can own subagents")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return a.childLocked(id)
}

func (a *Agent) childLocked(id bson.ObjectID) (*Agent, error) {
	m := a.runtime.main
	if m.closed || a.ctx.Err() != nil {
		return nil, errors.New("main agent is stopped")
	}
	if child := m.children[id]; child != nil {
		return child, nil
	}
	// Verify ownership even during recovery; IDs alone are not authorization.
	if _, err := a.base.Store.GetTaskStoreServer(bson.M{"parent": a.id, "task_id": id.Hex()}); err != nil {
		return nil, err
	}
	config := *a.base
	config.Tools = nil
	toolMap := make(map[string]tool.Tool)
	for _, t := range a.base.Tools {
		if t.Define().GetFunction().Name == subAgentToolName {
			continue
		}
		config.Tools = append(config.Tools, t)
		toolMap[t.Define().GetFunction().Name] = t
	}
	child := &Agent{
		id:      id,
		parent:  a.id,
		ctx:     a.ctx,
		base:    &config,
		prompt:  a.prompt,
		toolMap: toolMap,
		sub:     &subRuntime{},
		runtime: &Runtime{tasks: taskheap.NewManager()},
	}
	if err := child.Execute(""); err != nil {
		finishErr := a.base.Store.FinishSubAgentTask(a.id, id, tool.TaskFailed, err.Error())
		return nil, errors.Join(err, finishErr)
	}
	m.children[id] = child
	return child, nil
}

// Execute starts a previously persisted task. Its lifetime comes exclusively
// from the owning main agent, not the short-lived tool request context.
func (a *Agent) Execute(content string) error {
	if a.parent.IsZero() || a.sub == nil {
		return errors.New("subagent has no parent agent")
	}
	a.sub.mu.Lock()
	defer a.sub.mu.Unlock()
	if a.sub.started {
		return errors.New("subagent is already started")
	}
	state, err := a.base.Store.GetTaskStoreServer(bson.M{"parent": a.parent, "task_id": a.id.Hex()})
	if err != nil {
		return err
	}
	if terminalTask(state.Status) {
		return nil
	}
	if err := a.getHistoryMessages(); err != nil {
		return err
	}
	a.sub.started = true
	a.runtime.wg.Add(1)
	go func() {
		defer a.runtime.wg.Done()
		// Recover a final message committed immediately before a crash without
		// invoking the model again.
		var runErr error
		messages := a.runtime.OldMessages
		if len(messages) == 0 || messages[len(messages)-1].Role != provider.RoleAssistant || len(messages[len(messages)-1].ToolCalls) != 0 {
			runErr = a.runLoop()
		}
		status, result := tool.TaskCompleted, ""
		if runErr != nil {
			status, result = tool.TaskFailed, runErr.Error()
			if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
				status = tool.TaskCanceled
			}
		} else if n := len(a.runtime.OldMessages); n != 0 {
			result = a.runtime.OldMessages[n-1].Content
		}
		// Terminal writes must still work after the parent is canceled.
		store := a.base.Store
		ctx, cancel := context.WithTimeout(context.WithoutCancel(a.ctx), 5*time.Second)
		defer cancel()
		if dao, ok := store.(*db.Dao); ok {
			store = dao.WithContext(ctx)
		}
		err := store.FinishSubAgentTask(a.parent, a.id, status, result)
		a.sub.mu.Lock()
		a.sub.finishErr = err
		a.sub.mu.Unlock()
	}()
	return nil
}

type subAgentInput struct {
	ContextID string `json:"context_id"`
	TaskID    string `json:"task_id"`
	Content   string `json:"content"`
}

// Resume accepts targeted inputs, or a plain answer when exactly one tool is
// waiting. Polling stays in the existing child; no second runner is started.
func (a *Agent) Resume(content string) error {
	if a.parent.IsZero() || a.sub == nil {
		return errors.New("subagent has no parent agent")
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("input is required")
	}
	state, err := a.GetState()
	if err != nil {
		return err
	}
	if state.Status != tool.TaskInputRequired {
		return errors.New("subagent is not waiting for input")
	}
	waiting, err := a.waitingInputs()
	if err != nil {
		return err
	}
	var params struct {
		Inputs []subAgentInput `json:"inputs"`
	}
	if json.Unmarshal([]byte(content), &params) != nil || len(params.Inputs) == 0 {
		if len(waiting) != 1 {
			return errors.New("provide inputs with context_id, task_id and content for each waiting task")
		}
		params.Inputs = []subAgentInput{{ContextID: waiting[0].ContextId, TaskID: waiting[0].TaskId, Content: content}}
	}
	seen := make(map[string]bool)
	for _, input := range params.Inputs {
		found := false
		for _, pending := range waiting {
			if input.ContextID == pending.ContextId && input.TaskID == pending.TaskId {
				found = true
				break
			}
		}
		key := input.ContextID + "\x00" + input.TaskID
		if !found || seen[key] || strings.TrimSpace(input.Content) == "" {
			return errors.New("invalid or duplicate subagent input target")
		}
		seen[key] = true
	}
	inputs := make([]*schema.RemoteTaskStore, 0, len(params.Inputs))
	for _, input := range params.Inputs {
		inputs = append(inputs, &schema.RemoteTaskStore{ContextId: input.ContextID, TaskId: input.TaskID, Content: input.Content})
	}
	return a.base.Store.ResumeSubAgentTasks(a.id, inputs)
}

func (a *Agent) waitingInputs() ([]*schema.RemoteTaskStore, error) {
	return a.base.Store.ListRemoteTaskStores(bson.M{"conversation": a.id, "status": schema.RemoteTaskWaitingInput})
}

func (a *Agent) GetState() (*schema.TaskStoreServer, error) {
	if a.parent.IsZero() || a.sub == nil {
		return nil, errors.New("subagent has no parent agent")
	}
	a.sub.mu.Lock()
	finishErr := a.sub.finishErr
	a.sub.mu.Unlock()
	if finishErr != nil {
		return nil, fmt.Errorf("persist subagent result: %w", finishErr)
	}
	state, err := a.base.Store.GetTaskStoreServer(bson.M{"parent": a.parent, "task_id": a.id.Hex()})
	if err != nil || terminalTask(state.Status) {
		return state, err
	}
	c := *state
	state = &c
	waiting, err := a.waitingInputs()
	if err != nil {
		return nil, err
	}
	state.Status = tool.TaskWorking
	if len(waiting) != 0 {
		inputs := make([]subAgentInput, 0, len(waiting))
		for _, item := range waiting {
			inputs = append(inputs, subAgentInput{ContextID: item.ContextId, TaskID: item.TaskId, Content: item.Content})
		}
		body, _ := json.Marshal(map[string]any{"inputs": inputs})
		state.Status = tool.TaskInputRequired
		state.Artifacts = []string{string(body)}
	}
	return state, nil
}

func terminalTask(status tool.TaskStatus) bool {
	switch status {
	case tool.TaskCompleted, tool.TaskFailed, tool.TaskCanceled, tool.TaskRejected, tool.TaskAuthRequired:
		return true
	default:
		return false
	}
}
