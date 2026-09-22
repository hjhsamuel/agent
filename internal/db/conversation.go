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
	result, err := collection.InsertOne(d.context(), obj)
	if err != nil {
		return "", err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id.Hex(), nil
}

func (d *Dao) GetConversation(filter bson.M) (*schema.Conversation, error) {
	collection := d.getCollection(schema.ConversationCollection)
	result := collection.FindOne(d.context(), filter)
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
	cursor, err := collection.Find(d.context(), filter, opts...)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Conversation
	if err = cursor.All(d.context(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Dao) UpdateConversation(filter bson.M, update bson.M) error {
	collection := d.getCollection(schema.ConversationCollection)
	result, err := collection.UpdateOne(d.context(), filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// BeginConversation atomically claims an idle conversation and saves its input.
func (d *Dao) BeginConversation(id bson.ObjectID, user, content string) error {
	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(d.context())
	_, err = session.WithTransaction(d.context(), func(ctx context.Context) (any, error) {
		var conversation schema.Conversation
		if err := d.getCollection(schema.ConversationCollection).FindOne(ctx, bson.M{"_id": id, "user": user}).Decode(&conversation); err != nil {
			return nil, err
		}
		// Repair failed conversations left with pending tools by older versions.
		// This transaction also saves the next user input, preserving message order.
		if conversation.Status == schema.ConversationFailed {
			if err := d.closeActiveTools(ctx, &conversation, "Previous local execution failed or was canceled."); err != nil {
				return nil, err
			}
		}
		result, err := d.getCollection(schema.ConversationCollection).UpdateOne(
			ctx,
			bson.M{
				"_id":  id,
				"user": user,
				"status": bson.M{
					"$in": []schema.ConversationState{
						schema.ConversationTemp,
						schema.ConversationDone,
						schema.ConversationFailed,
					},
				},
				"active_tools.0": bson.M{"$exists": false},
			},
			bson.M{"$set": bson.M{"status": schema.ConversationActive}})
		if err != nil {
			return nil, err
		}
		if result.MatchedCount == 0 {
			return nil, mongo.ErrNoDocuments
		}
		_, err = d.getCollection(schema.MessageCollection).InsertOne(ctx, &schema.Message{Conversation: id, Role: provider.RoleUser, Content: content})
		return nil, err
	})
	return err
}

// FinishConversation commits terminal state and paired tool results together.
// Its context must belong to the service, not to the canceled runner.
func (d *Dao) FinishConversation(id bson.ObjectID, state schema.ConversationState, reason string) error {
	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(d.context())
	_, err = session.WithTransaction(d.context(), func(ctx context.Context) (any, error) {
		var conversation schema.Conversation
		if err := d.getCollection(schema.ConversationCollection).FindOne(ctx, bson.M{"_id": id}).Decode(&conversation); err != nil {
			return nil, err
		}
		if state == schema.ConversationFailed {
			if err := d.closeActiveTools(ctx, &conversation, reason); err != nil {
				return nil, err
			}
		}
		_, err := d.getCollection(schema.ConversationCollection).UpdateOne(ctx,
			bson.M{"_id": id}, bson.M{"$set": bson.M{"status": state}})
		return nil, err
	})
	return err
}

func (d *Dao) closeActiveTools(ctx context.Context, conversation *schema.Conversation, reason string) error {
	if len(conversation.ActiveTools) == 0 {
		return nil
	}
	content := "Local tool waiting stopped: " + reason +
		" Remote execution may still be running or may already have completed; its outcome is unknown. Do not automatically repeat the operation."
	messages := make([]*schema.Message, 0, len(conversation.ActiveTools))
	callIDs := make([]string, 0, len(conversation.ActiveTools))
	for _, active := range conversation.ActiveTools {
		messages = append(messages, &schema.Message{Conversation: conversation.ID, Role: provider.RoleTool, ToolCallId: active.ToolCallId, Content: content})
		callIDs = append(callIDs, active.ToolCallId)
	}
	if _, err := d.getCollection(schema.MessageCollection).InsertMany(ctx, messages); err != nil {
		return err
	}
	// RemoteTaskDone ends local polling only; content explicitly records uncertainty.
	if _, err := d.getCollection(schema.RemoteTaskStoreCollection).UpdateMany(ctx,
		bson.M{"conversation": conversation.ID, "tool.tool_call_id": bson.M{"$in": callIDs}},
		bson.M{"$set": bson.M{"status": schema.RemoteTaskDone, "content": content}}); err != nil {
		return err
	}
	_, err := d.getCollection(schema.ConversationCollection).UpdateOne(ctx,
		bson.M{"_id": conversation.ID}, bson.M{"$set": bson.M{"active_tools": bson.A{}}})
	return err
}

func (d *Dao) AddConversationMessage(objs ...*schema.Message) error {
	collection := d.getCollection(schema.MessageCollection)
	_, err := collection.InsertMany(d.context(), objs)
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
	cursor, err := collection.Find(d.context(), filter, opts...)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Message
	if err = cursor.All(d.context(), &objs); err != nil {
		return nil, err
	}
	return objs, nil
}

func (d *Dao) GetConversationCompaction(
	filter bson.M,
	opts ...options.Lister[options.FindOneOptions],
) (*schema.Compaction, error) {
	collection := d.getCollection(schema.CompactCollection)
	result := collection.FindOne(d.context(), filter, opts...)
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
	cursor, err := collection.Find(d.context(), filter)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Compaction
	if err = cursor.All(d.context(), &objs); err != nil {
		return nil, err
	}
	return objs, nil
}

func (d *Dao) AddConversationCompaction(obj *schema.Compaction) error {
	collection := d.getCollection(schema.CompactCollection)
	_, err := collection.InsertOne(d.context(), obj)
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

	ctx := d.context()

	session, err := d.StartSession()
	if err != nil {
		return bson.ObjectID{}, err
	}
	defer session.EndSession(ctx)

	var id bson.ObjectID
	_, err = session.WithTransaction(d.context(), func(sc context.Context) (any, error) {
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

	ctx := d.context()

	session, err := d.StartSession()
	if err != nil {
		return id, err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(d.context(), func(sc context.Context) (any, error) {
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
