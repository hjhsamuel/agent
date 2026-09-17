package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/provider"
	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Dao) CreateRemoteTaskStore(obj *schema.RemoteTaskStore) error {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	_, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) GetRemoteTaskStore(filter bson.M) (*schema.RemoteTaskStore, error) {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	result := collection.FindOne(context.Background(), filter)

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
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var objs []*schema.RemoteTaskStore
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Dao) UpdateRemoteTaskStore(filter bson.M, update bson.M) error {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) CreateTaskStoreServer(content string) (bson.ObjectID, error) {
	var (
		conversationColl = d.getCollection(schema.ConversationCollection)
		messageColl      = d.getCollection(schema.MessageCollection)
		taskStoreColl    = d.getCollection(schema.TaskStoreServerCollection)

		id bson.ObjectID
	)

	ctx := context.Background()

	session, err := d.StartSession()
	if err != nil {
		return bson.ObjectID{}, err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
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

func (d *Dao) GetTaskStoreServer(filter bson.M) (*schema.TaskStoreServer, error) {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	result := collection.FindOne(context.Background(), filter)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.TaskStoreServer
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Dao) UpdateTaskStoreServer(filter bson.M, update bson.M) (int64, error) {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	result, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}
