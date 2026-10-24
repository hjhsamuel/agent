package store

import (
	"context"
	"errors"
	"time"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (d *Store) AddAgent(
	kind schema.AgentType,
	parent *bson.ObjectID,
	info *request.AgentBase,
	tags ...bson.ObjectID,
) (bson.ObjectID, error) {
	var (
		result *mongo.InsertOneResult
		err    error
	)
	switch kind {
	case schema.AgentTypeMain:
		result, err = d.getCollection(schema.AgentCollection).InsertOne(
			context.Background(),
			&schema.Agent{
				Name:        info.Name,
				Description: info.Description,
				Prompt:      info.Prompt,
				Type:        schema.AgentTypeMain,
				Tag:         tags,
				CreatedAt:   time.Now(),
			},
		)
	case schema.AgentTypeRunner:
		result, err = d.getCollection(schema.AgentCollection).InsertOne(
			context.Background(),
			&schema.Agent{
				Name:        info.Name,
				Description: info.Description,
				Prompt:      info.Prompt,
				Type:        schema.AgentTypeRunner,
				Parent:      parent,
				Tag:         tags,
				CreatedAt:   time.Now(),
			},
		)
	case schema.AgentTypeGroup:
		result, err = d.getCollection(schema.AgentCollection).InsertOne(
			context.Background(),
			&schema.Agent{
				Name:        info.Name,
				Description: info.Description,
				Prompt:      info.Prompt,
				Type:        schema.AgentTypeGroup,
				Tag:         tags,
				CreatedAt:   time.Now(),
			},
		)
	default:
		return bson.ObjectID{}, errors.New("unknown agent type")
	}
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
