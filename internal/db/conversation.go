package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Dao) CreateConversation(obj *schema.Conversation) (string, error) {
	collection := d.getCollection(schema.ConversationCollection)
	result, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return "", err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id.String(), nil
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

func (d *Dao) GetConversationCompaction(filter bson.M) (*schema.Compaction, error) {
	collection := d.getCollection(schema.CompactCollection)
	result := collection.FindOne(context.Background(), filter)
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
	collection := d.getCollection(schema.ConversationCollection)
	_, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return err
	}
	return nil
}
