package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Dao) CreateRemoteTaskStore(obj *schema.RemoteTaskStore) error {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	_, err := collection.InsertOne(d.context(), obj)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) GetRemoteTaskStore(filter bson.M) (*schema.RemoteTaskStore, error) {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	result := collection.FindOne(d.context(), filter)

	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.RemoteTaskStore
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Dao) ListRemoteTaskStores(filter bson.M) ([]*schema.RemoteTaskStore, error) {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	cursor, err := collection.Find(d.context(), filter)
	if err != nil {
		return nil, err
	}

	var objs []*schema.RemoteTaskStore
	if err = cursor.All(d.context(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Dao) UpdateRemoteTaskStore(filter bson.M, update bson.M) error {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	result, err := collection.UpdateOne(d.context(), filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Dao) CreateTaskStoreServer(content string) (bson.ObjectID, error) {
	return d.createTaskStoreServer(bson.NilObjectID, bson.NilObjectID, "", content)
}

// CreateSubAgentTask commits the child and its parent's pending tool together.
// Recovery can therefore find the child without repeating delegated work.
func (d *Dao) CreateSubAgentTask(parent, message bson.ObjectID, callID, content string) (bson.ObjectID, error) {
	return d.createTaskStoreServer(parent, message, callID, content)
}

func (d *Dao) createTaskStoreServer(parent, message bson.ObjectID, callID, content string) (bson.ObjectID, error) {
	var (
		conversationColl = d.getCollection(schema.ConversationCollection)
		messageColl      = d.getCollection(schema.MessageCollection)
		taskStoreColl    = d.getCollection(schema.TaskStoreServerCollection)

		id bson.ObjectID
	)

	ctx := d.context()

	session, err := d.StartSession()
	if err != nil {
		return bson.ObjectID{}, err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		if !parent.IsZero() {
			var existing schema.RemoteTaskStore
			err := d.getCollection(schema.RemoteTaskStoreCollection).FindOne(sc, bson.M{
				"conversation": parent, "tool.tool_call_id": callID,
			}).Decode(&existing)
			if err == nil {
				id, err = bson.ObjectIDFromHex(existing.TaskId)
				return nil, err
			}
			if err != mongo.ErrNoDocuments {
				return nil, err
			}
		}
		// 会话
		result, err := conversationColl.InsertOne(sc, &schema.Conversation{
			Status: schema.ConversationActive,
		})
		if err != nil {
			return nil, err
		}

		id = result.InsertedID.(bson.ObjectID)
		// 消息
		_, err = messageColl.InsertOne(sc, &schema.Message{
			Conversation: id,
			Role:         provider.RoleUser,
			Content:      content,
		})
		if err != nil {
			return nil, err
		}
		// 任务
		_, err = taskStoreColl.InsertOne(sc, &schema.TaskStoreServer{
			Parent:    parent,
			ContextId: id.Hex(),
			TaskId:    id.Hex(),
			Status:    tool.TaskSubmitted,
			History: []*schema.TaskMessage{
				{Role: schema.TaskRoleUser, Content: content},
			},
			Version: 1,
		})
		if err != nil {
			return nil, err
		}
		if !parent.IsZero() {
			_, err = d.getCollection(schema.RemoteTaskStoreCollection).InsertOne(sc, &schema.RemoteTaskStore{
				Conversation: parent, Message: message,
				ContextId: id.Hex(), TaskId: id.Hex(), Status: schema.RemoteTaskSubmitted,
				Tool: &schema.TaskTool{ToolCallId: callID, Name: "subagent"},
			})
			if err != nil {
				return nil, err
			}
		}

		return nil, nil
	},
		options.Transaction().
			SetReadConcern(nil).
			SetWriteConcern(nil),
	)
	if err != nil {
		return bson.ObjectID{}, err
	}

	return id, nil
}

// FinishSubAgentTask publishes the result and closes the private conversation
// in one transaction. Only the owning main agent may finish this task.
func (d *Dao) FinishSubAgentTask(parent, id bson.ObjectID, status tool.TaskStatus, content string) error {
	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(d.context())
	_, err = session.WithTransaction(d.context(), func(ctx context.Context) (any, error) {
		result, err := d.getCollection(schema.TaskStoreServerCollection).UpdateOne(ctx,
			bson.M{"parent": parent, "task_id": id.Hex()},
			bson.M{"$set": bson.M{"status": status, "artifacts": []string{content}}, "$inc": bson.M{"version": 1}})
		if err != nil {
			return nil, err
		}
		if result.MatchedCount == 0 {
			return nil, mongo.ErrNoDocuments
		}
		state := schema.ConversationDone
		if status != tool.TaskCompleted {
			state = schema.ConversationFailed
			var conversation schema.Conversation
			if err := d.getCollection(schema.ConversationCollection).FindOne(ctx, bson.M{"_id": id}).Decode(&conversation); err != nil {
				return nil, err
			}
			if err := d.closeActiveTools(ctx, &conversation, content); err != nil {
				return nil, err
			}
		}
		_, err = d.getCollection(schema.ConversationCollection).UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": state}})
		return nil, err
	})
	return err
}

func (d *Dao) GetTaskStoreServer(filter bson.M) (*schema.TaskStoreServer, error) {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	result := collection.FindOne(d.context(), filter)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.TaskStoreServer
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

// ResumeSubAgentTasks applies a batch atomically so a retry cannot strand a
// partially resumed group of input_required tools.
func (d *Dao) ResumeSubAgentTasks(conversation bson.ObjectID, inputs []*schema.RemoteTaskStore) error {
	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(d.context())
	_, err = session.WithTransaction(d.context(), func(ctx context.Context) (any, error) {
		for _, input := range inputs {
			result, err := d.getCollection(schema.RemoteTaskStoreCollection).UpdateOne(ctx,
				bson.M{"conversation": conversation, "context_id": input.ContextId, "task_id": input.TaskId, "status": schema.RemoteTaskWaitingInput},
				bson.M{"$set": bson.M{"status": schema.RemoteTaskInputted, "content": input.Content}})
			if err != nil {
				return nil, err
			}
			if result.MatchedCount == 0 {
				return nil, mongo.ErrNoDocuments
			}
		}
		return nil, nil
	})
	return err
}

func (d *Dao) UpdateTaskStoreServer(filter bson.M, update bson.M) (int64, error) {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	result, err := collection.UpdateOne(d.context(), filter, update)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}
