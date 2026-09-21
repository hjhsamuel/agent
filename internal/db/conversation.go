package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Dao) CreateConversation(obj *schema.Conversation) (string, error) {
	collection := d.getCollection(schema.ConversationCollection)
	result, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return "", err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id.Hex(), nil
}

func (d *Dao) GetConversation(filter bson.M) (*schema.Conversation, error) {
	collection := d.getCollection(schema.ConversationCollection)
	result := collection.FindOne(context.Background(), filter)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Conversation
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

func (d *Dao) ListConversations(
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Conversation, error) {
	collection := d.getCollection(schema.ConversationCollection)
	cursor, err := collection.Find(context.Background(), filter, opts...)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Conversation
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Dao) UpdateConversation(filter bson.M, update bson.M) error {
	collection := d.getCollection(schema.ConversationCollection)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) AddConversationMessage(objs ...*schema.Message) error {
	collection := d.getCollection(schema.MessageCollection)
	_, err := collection.InsertMany(context.Background(), objs)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) GetConversationMessages(
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Message, error) {
	collection := d.getCollection(schema.MessageCollection)
	cursor, err := collection.Find(context.Background(), filter, opts...)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Message
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}
	return objs, nil
}

func (d *Dao) GetConversationCompaction(
	filter bson.M,
	opts ...options.Lister[options.FindOneOptions],
) (*schema.Compaction, error) {
	collection := d.getCollection(schema.CompactCollection)
	result := collection.FindOne(context.Background(), filter, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Compaction
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Dao) ListConversationCompactions(filter bson.M) ([]*schema.Compaction, error) {
	collection := d.getCollection(schema.CompactCollection)
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Compaction
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}
	return objs, nil
}

func (d *Dao) AddConversationCompaction(obj *schema.Compaction) error {
	collection := d.getCollection(schema.CompactCollection)
	_, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) ActiveTaskFinished(
	conversationId bson.ObjectID,
	tasks ...*schema.FinishActiveReq,
) (bson.ObjectID, error) {
	var (
		newMessages   = make([]*schema.Message, 0, len(tasks))
		pullIds       = make([]string, 0, len(tasks))
		remoteUpdates = make([]mongo.WriteModel, 0)
	)
	for _, task := range tasks {
		newMessages = append(newMessages, &schema.Message{
			Conversation: conversationId,
			Role:         provider.RoleTool,
			Content:      task.Content,
			ToolCallId:   task.ToolCallId,
		})

		pullIds = append(pullIds, task.ToolCallId)

		if task.TaskId != "" {
			remoteUpdates = append(remoteUpdates, mongo.NewUpdateOneModel().
				SetFilter(bson.M{
					"context_id":        task.TaskId,
					"task_id":           task.TaskId,
					"conversation":      conversationId,
					"tool.tool_call_id": task.ToolCallId,
				}).
				SetUpdate(bson.M{
					"$set": bson.M{
						"status":  schema.RemoteTaskDone,
						"content": task.Content,
					},
				}),
			)
		}
	}

	var (
		conversationColl = d.getCollection(schema.ConversationCollection)
		messageColl      = d.getCollection(schema.MessageCollection)
		remoteTaskColl   = d.getCollection(schema.RemoteTaskStoreCollection)
	)

	ctx := context.Background()

	session, err := d.StartSession()
	if err != nil {
		return bson.ObjectID{}, err
	}
	defer session.EndSession(ctx)

	var id bson.ObjectID
	_, err = session.WithTransaction(context.Background(), func(sc context.Context) (any, error) {
		// 添加消息
		result, err := messageColl.InsertMany(sc, newMessages)
		if err != nil {
			return nil, err
		}
		id = result.InsertedIDs[len(result.InsertedIDs)-1].(bson.ObjectID)

		// 移除任务记录
		_, err = conversationColl.UpdateOne(
			sc,
			bson.M{"_id": conversationId},
			bson.M{"$pull": bson.M{"active_tools": bson.M{"tool_call_id": bson.M{"$in": pullIds}}}},
		)
		if err != nil {
			return nil, err
		}

		// 可能需要修改异步任务信息
		if len(remoteUpdates) != 0 {
			_, err = remoteTaskColl.BulkWrite(sc, remoteUpdates)
			if err != nil {
				return nil, err
			}
		}

		return nil, nil
	})
	if err != nil {
		return bson.ObjectID{}, err
	}
	return id, nil
}

func (d *Dao) AddMessageWithToolCalls(
	message *schema.Message,
	toolCalls ...*schema.ActiveTool,
) (bson.ObjectID, error) {
	var (
		conversationColl = d.getCollection(schema.ConversationCollection)
		messageColl      = d.getCollection(schema.MessageCollection)
		id               bson.ObjectID
	)

	ctx := context.Background()

	session, err := d.StartSession()
	if err != nil {
		return id, err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(context.Background(), func(sc context.Context) (any, error) {
		// 添加消息
		result, err := messageColl.InsertOne(sc, message)
		if err != nil {
			return nil, err
		}

		id = result.InsertedID.(bson.ObjectID)

		for _, item := range toolCalls {
			item.Message = id
		}

		// 设置任务
		_, err = conversationColl.UpdateOne(
			sc,
			bson.M{"_id": message.Conversation},
			bson.M{"$push": bson.M{"active_tools": bson.M{"$each": toolCalls}}},
		)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		return id, err
	}

	return id, nil
}
