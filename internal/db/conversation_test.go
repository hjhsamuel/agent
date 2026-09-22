package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Requires a transaction-capable replica set. Every run owns an isolated database.
func TestFailedConversationCanStartAgain(t *testing.T) {
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
	for _, legacy := range []bool{false, true} {
		name := "canceled"
		if legacy {
			name = "legacy_failed"
		}
		t.Run(name, func(t *testing.T) {
			id := bson.NewObjectID()
			state := schema.ConversationActive
			if legacy {
				state = schema.ConversationFailed
			}
			if _, err := d.CreateConversation(&schema.Conversation{ID: id, User: "1", Status: state}); err != nil {
				t.Fatal(err)
			}
			calls := []*schema.ActiveTool{{ToolCallId: "sync", Name: "echo"}, {ToolCallId: "async", Name: "remote"}}
			message := &schema.Message{Conversation: id, Role: provider.RoleAssistant, ToolCalls: []*schema.ToolCall{{ID: "sync", Name: "echo"}, {ID: "async", Name: "remote"}}}
			if _, err := d.AddMessageWithToolCalls(message, calls...); err != nil {
				t.Fatal(err)
			}
			if err := d.CreateRemoteTaskStore(&schema.RemoteTaskStore{Conversation: id, TaskId: "task", Status: schema.RemoteTaskWaitingInput, Tool: &schema.TaskTool{ToolCallId: "async", Name: "remote"}}); err != nil {
				t.Fatal(err)
			}
			if !legacy {
				// Repeated completion must not append duplicate tool results.
				for range 2 {
					if err := d.FinishConversation(id, schema.ConversationFailed, context.Canceled.Error()); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := d.BeginConversation(id, "other-user", "unauthorized"); err == nil {
				t.Fatal("another user started the conversation")
			}
			if err := d.BeginConversation(id, "1", "continue"); err != nil {
				t.Fatal(err)
			}
			conversation, err := d.GetConversation(bson.M{"_id": id})
			if err != nil || conversation.Status != schema.ConversationActive || len(conversation.ActiveTools) != 0 {
				t.Fatalf("conversation not reusable: %+v, %v", conversation, err)
			}
			messages, err := d.GetConversationMessages(bson.M{"conversation": id}, options.Find().SetSort(bson.M{"_id": 1}))
			if err != nil || len(messages) != 4 {
				t.Fatalf("expected assistant, two tool results and new input: %+v, %v", messages, err)
			}
			for i, call := range calls {
				result := messages[i+1]
				if result.Role != provider.RoleTool || result.ToolCallId != call.ToolCallId || !strings.Contains(result.Content, "outcome is unknown") {
					t.Fatalf("missing paired interruption result: %+v", result)
				}
			}
			if messages[3].Role != provider.RoleUser || messages[3].Content != "continue" {
				t.Fatal("new input precedes pending tool results")
			}
			task, err := d.GetRemoteTaskStore(bson.M{"conversation": id, "task_id": "task"})
			if err != nil || task.Status != schema.RemoteTaskDone || !strings.Contains(task.Content, "outcome is unknown") {
				t.Fatalf("local task still waiting: %+v, %v", task, err)
			}
		})
	}
}
