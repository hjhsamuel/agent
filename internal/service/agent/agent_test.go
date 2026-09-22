package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/entities"
	"github.com/hjhsamuel/agent/internal/llm"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/agent/resolver"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type memoryStore struct {
	*db.Dao    // Unexpected persistence calls fail the test instead of succeeding silently.
	finalErr   error
	final      *schema.Message
	compact    *schema.Compaction
	lastResult bson.ObjectID
	updates    []bson.M
}

func (s *memoryStore) BeginConversation(bson.ObjectID, string, string) error { return nil }
func (s *memoryStore) GetConversation(bson.M) (*schema.Conversation, error) {
	return &schema.Conversation{}, nil
}
func (s *memoryStore) GetConversationCompaction(bson.M, ...options.Lister[options.FindOneOptions]) (*schema.Compaction, error) {
	return nil, mongo.ErrNoDocuments
}
func (s *memoryStore) GetConversationMessages(bson.M, ...options.Lister[options.FindOptions]) ([]*schema.Message, error) {
	return []*schema.Message{{ID: bson.NewObjectID(), Role: provider.RoleUser, Content: "hello"}}, nil
}
func (s *memoryStore) AddConversationMessage(messages ...*schema.Message) error {
	s.final = messages[0]
	return s.finalErr
}
func (s *memoryStore) AddMessageWithToolCalls(*schema.Message, ...*schema.ActiveTool) (bson.ObjectID, error) {
	return bson.NewObjectID(), nil
}
func (s *memoryStore) ActiveTaskFinished(bson.ObjectID, ...*schema.FinishActiveReq) (bson.ObjectID, error) {
	s.lastResult = bson.NewObjectID()
	return s.lastResult, nil
}
func (s *memoryStore) AddConversationCompaction(c *schema.Compaction) error {
	s.compact = c
	return nil
}
func (s *memoryStore) UpdateRemoteTaskStore(_ bson.M, update bson.M) error {
	s.updates = append(s.updates, update)
	return nil
}

type modelStub struct {
	stream func([]*provider.Message) (*provider.Message, error)
	chat   func([]*provider.Message) (*provider.Message, error)
}

func (m modelStub) Stream(_ context.Context, _, _ string, messages []*provider.Message, _ *provider.ChatConfig, _ provider.YieldFunc) (*provider.Message, error) {
	return m.stream(messages)
}
func (m modelStub) Chat(_ context.Context, _, _ string, messages []*provider.Message, _ *provider.ChatConfig) (*provider.Message, error) {
	return m.chat(messages)
}

type echoTool struct{ tool.Tool }

func (echoTool) Define() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{OfFunction: &openai.ChatCompletionFunctionToolParam{Function: shared.FunctionDefinitionParam{Name: "echo"}}}
}
func (echoTool) Execute(context.Context, string, string) (*tool.ToolResult, error) {
	return &tool.ToolResult{Content: "result"}, nil
}

func testAgent(t *testing.T, store *memoryStore, model modelStub) *Agent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m := llm.New("test", nil, model)
	return NewAgent(ctx, bson.NewObjectID(), &BaseConfig{User: &entities.UserInfo{ID: 1}, Store: store, Provider: m, Compact: m, Tools: []tool.Tool{echoTool{}}, Up: make(chan *notify.UpperEvent, 32), Shutdown: ctx.Done()}).(*Agent)
}

func TestSimpleConversationWithoutUsageAndPersistenceError(t *testing.T) {
	for _, saveErr := range []error{nil, errors.New("write failed")} {
		t.Run(fmt.Sprint(saveErr), func(t *testing.T) {
			store := &memoryStore{finalErr: saveErr}
			a := testAgent(t, store, modelStub{stream: func(messages []*provider.Message) (*provider.Message, error) {
				if len(messages) != 1 || messages[0].Role != provider.RoleUser {
					t.Errorf("unexpected history: %v", messages)
				}
				return &provider.Message{Role: provider.RoleAssistant, Content: "hello"}, nil
			}})
			if err := a.Start("hello"); err != nil {
				t.Fatal(err)
			}
			a.Wait()
			select {
			case event := <-a.base.Up:
				if !event.Finished || !errors.Is(event.Error, saveErr) {
					t.Fatalf("wrong terminal event: %+v", event)
				}
			default:
				t.Fatal("missing terminal event")
			}
		})
	}
}

func TestToolRoundsHavePairedMessagesWithoutDuplicates(t *testing.T) {
	step := 0
	a := testAgent(t, &memoryStore{}, modelStub{stream: func(messages []*provider.Message) (*provider.Message, error) {
		if len(messages) != 1+2*step {
			t.Fatalf("round %d has %d messages", step, len(messages))
		}
		for i := 1; i < len(messages); i += 2 {
			if messages[i].Role != provider.RoleAssistant || messages[i+1].Role != provider.RoleTool || messages[i].ToolCalls[0].ID != messages[i+1].ToolCallId {
				t.Fatal("unpaired tool response")
			}
		}
		step++
		if step == 3 {
			return &provider.Message{Role: provider.RoleAssistant, Content: "done"}, nil
		}
		return &provider.Message{Role: provider.RoleAssistant, ToolCalls: []*provider.ToolCall{{ID: fmt.Sprint(step), Name: "echo", Arguments: "{}"}}}, nil
	}})
	if err := a.Start("hello"); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	if event := <-a.base.Up; event.Error != nil {
		t.Fatal(event.Error)
	}
	if step != 3 {
		t.Fatalf("got %d turns", step)
	}
}

func TestCompactionIncludesCompletedToolResult(t *testing.T) {
	store := &memoryStore{}
	step := 0
	a := testAgent(t, store, modelStub{
		chat: func(messages []*provider.Message) (*provider.Message, error) {
			return &provider.Message{Role: provider.RoleAssistant, Content: "summary"}, nil
		},
		stream: func(messages []*provider.Message) (*provider.Message, error) {
			step++
			if step == 1 {
				return &provider.Message{Role: provider.RoleAssistant, Usage: &provider.TokenUsage{Prompt: 200}, ToolCalls: []*provider.ToolCall{{ID: "1", Name: "echo"}}}, nil
			}
			if len(messages) != 1 || messages[0].Content != "summary" {
				t.Fatal("orphaned tool response after compaction")
			}
			return &provider.Message{Role: provider.RoleAssistant, Content: "done"}, nil
		},
	})
	a.base.Provider.Capabilities = &schema.ModelCapabilities{ContextLimit: 200}
	if err := a.Start("hello"); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	if event := <-a.base.Up; event.Error != nil {
		t.Fatal(event.Error)
	}
	if store.compact == nil || store.compact.Message != store.lastResult {
		t.Fatal("summary checkpoint excludes tool result")
	}
}

func TestResolverIgnoresUnknownDuplicateAndRequeuesOmissions(t *testing.T) {
	store := &memoryStore{}
	a := testAgent(t, store, modelStub{chat: func([]*provider.Message) (*provider.Message, error) {
		return &provider.Message{Content: `{"results":[null,{"tool_call_id":"unknown","action":"ask_user","question":"?"},{"tool_call_id":"a","action":"provide_input","input":{"x":1}},{"tool_call_id":"a","action":"ask_user","question":"duplicate"}]}`}, nil
	}})
	items := []*resolver.InputRequiredItem{{TaskId: "1", ToolCallId: "a"}, {TaskId: "2", ToolCallId: "b"}}
	if err := a.resolveInputRequired(items...); err != nil {
		t.Fatal(err)
	}
	if a.runtime.tasks.Len() != 2 || len(store.updates) != 1 {
		t.Fatal("lost tasks or duplicated updates")
	}
	if _, ok := store.updates[0]["$set"].(bson.M)["content"].(string); !ok {
		t.Fatal("input stored as BSON binary")
	}
}

func TestShutdownUnblocksFullNotificationChannel(t *testing.T) {
	a := testAgent(t, &memoryStore{}, modelStub{})
	a.base.Up = make(chan *notify.UpperEvent)
	shutdown := make(chan struct{})
	a.base.Shutdown = shutdown
	done := make(chan struct{})
	go func() { a.loopDefer(context.Canceled); close(done) }()
	close(shutdown)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("exit blocked on notification")
	}
}

func TestModelTurnBudgetStopsRepeatedTools(t *testing.T) {
	a := testAgent(t, &memoryStore{}, modelStub{stream: func([]*provider.Message) (*provider.Message, error) {
		return &provider.Message{Role: provider.RoleAssistant, ToolCalls: []*provider.ToolCall{{ID: "tool", Name: "echo"}}}, nil
	}})
	a.base.MaxTurns = 2
	if err := a.Start("hello"); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	if event := <-a.base.Up; event.Error == nil {
		t.Fatal("unbounded tool loop")
	}
}
