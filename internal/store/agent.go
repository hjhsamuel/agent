package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Store) AddAgent(obj *schema.Agent) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.AgentCollection).InsertOne(context.Background(), obj)
	if err != nil {
		return bson.ObjectID{}, err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id, nil
}

func (d *Store) GetAgents(kind schema.AgentType) ([]*schema.Agent, error) {
	ctx := context.Background()
	cursor, err := d.getCollection(schema.AgentCollection).Find(ctx, bson.M{"type": kind})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []*schema.Agent
	if err = cursor.All(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (d *Store) AllAgent() ([]*schema.Agent, error) {
	ctx := context.Background()
	cursor, err := d.getCollection(schema.AgentCollection).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Agent
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Store) FindAgents(filter bson.M) ([]*schema.Agent, error) {
	ctx := context.Background()

	cursor, err := d.getCollection(schema.AgentCollection).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Agent
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}

	return objs, nil
}
