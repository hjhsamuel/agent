package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestSubAgentTaskTransactions(t *testing.T) {
	uri := os.Getenv("AGENT_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set AGENT_TEST_MONGO_URI to run MongoDB transaction integration tests")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	d := &Dao{db: "agent_test_" + bson.NewObjectID().Hex(), ctx: ctx, Client: client}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := client.Database(d.db).Drop(cleanup); err != nil {
			t.Error(err)
		}
		_ = client.Disconnect(cleanup)
	}()
	parent, message := bson.NewObjectID(), bson.NewObjectID()
	id, err := d.CreateSubAgentTask(parent, message, "call", "delegated task")
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := d.CreateSubAgentTask(parent, message, "call", "delegated task")
	if err != nil || repeated != id {
		t.Fatalf("creation replayed: %s, %v", repeated.Hex(), err)
	}
	state, err := d.GetTaskStoreServer(bson.M{"parent": parent, "task_id": id.Hex()})
	if err != nil || state.Status != tool.TaskSubmitted {
		t.Fatalf("task missing: %+v, %v", state, err)
	}
	pending, err := d.GetRemoteTaskStore(bson.M{"conversation": parent, "tool.tool_call_id": "call"})
	if err != nil || pending.TaskId != id.Hex() || pending.Message != message {
		t.Fatalf("parent mapping missing: %+v, %v", pending, err)
	}
	history, err := d.GetConversationMessages(bson.M{"conversation": id})
	if err != nil || len(history) != 1 || history[0].Content != "delegated task" {
		t.Fatalf("child history missing: %+v, %v", history, err)
	}
	for _, taskID := range []string{"one", "two"} {
		if err := d.CreateRemoteTaskStore(&schema.RemoteTaskStore{Conversation: id, ContextId: "ctx", TaskId: taskID, Status: schema.RemoteTaskWaitingInput}); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []*schema.RemoteTaskStore{{ContextId: "ctx", TaskId: "one", Content: "first"}, {ContextId: "ctx", TaskId: "missing", Content: "second"}}
	if err := d.ResumeSubAgentTasks(id, inputs); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("bad target accepted: %v", err)
	}
	first, err := d.GetRemoteTaskStore(bson.M{"conversation": id, "task_id": "one"})
	if err != nil || first.Status != schema.RemoteTaskWaitingInput {
		t.Fatal("failed input batch was partially applied")
	}
	inputs[1].TaskId = "two"
	if err := d.ResumeSubAgentTasks(id, inputs); err != nil {
		t.Fatal(err)
	}
	if err := d.FinishSubAgentTask(bson.NewObjectID(), id, tool.TaskCompleted, "foreign"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("foreign parent finished task: %v", err)
	}
	if err := d.FinishSubAgentTask(parent, id, tool.TaskCompleted, "result"); err != nil {
		t.Fatal(err)
	}
	if err := d.FinishSubAgentTask(parent, id, tool.TaskFailed, "late cleanup"); err != nil {
		t.Fatal(err)
	}
	state, err = d.GetTaskStoreServer(bson.M{"parent": parent, "task_id": id.Hex()})
	if err != nil || state.Status != tool.TaskCompleted || len(state.Artifacts) != 1 || state.Artifacts[0] != "result" {
		t.Fatalf("result not committed: %+v, %v", state, err)
	}
	conversation, err := d.GetConversation(bson.M{"_id": id})
	if err != nil || conversation.Status != schema.ConversationDone {
		t.Fatalf("child conversation not closed: %+v, %v", conversation, err)
	}
}
