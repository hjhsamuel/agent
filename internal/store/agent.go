package store

import (
	"context"
	"errors"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

func (d *Store) FindAgents(
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Agent, error) {
	ctx := context.Background()

	cursor, err := d.getCollection(schema.AgentCollection).Find(ctx, filter, opts...)
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

func (d *Store) UpdateAgent(filter, update bson.M) error {
	result, err := d.getCollection(schema.AgentCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	if result.ModifiedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Store) DeleteAgent(id bson.ObjectID) error {
	ctx := context.Background()

	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		agents := d.getCollection(schema.AgentCollection)

		var agent struct {
			Type schema.AgentType `bson:"type"`
		}
		if err = agents.FindOne(
			sc,
			bson.M{"_id": id},
			options.FindOne().SetProjection(bson.M{"type": 1, "_id": 0}),
		).Decode(&agent); err != nil {
			return nil, err
		}

		if agent.Type == schema.AgentTypeMain {
			return nil, errors.New("cannot delete main agent")
		}
		if agent.Type == schema.AgentTypeGroup {
			err = agents.FindOne(
				sc,
				bson.M{"parent": id},
				options.FindOne().SetProjection(bson.M{"_id": 1}),
			).Err()
			if err == nil {
				return nil, errors.New("cannot delete group with child agents")
			}
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, err
			}
		}

		result, err := agents.DeleteOne(sc, bson.M{"_id": id, "type": bson.M{"$ne": schema.AgentTypeMain}})
		if err != nil {
			return nil, err
		}
		if result.DeletedCount == 0 {
			return nil, mongo.ErrNoDocuments
		}

		// Index tool.agent so only associated tools are visited; retain the tools themselves.
		_, err = d.getCollection(schema.ToolCollection).UpdateMany(
			sc,
			bson.M{"agent": id},
			bson.M{
				"$unset": bson.M{"agent": ""},
				"$set":   bson.M{"state": schema.ToolStatusOffline},
			},
		)
		return nil, err
	})
	return err
}
