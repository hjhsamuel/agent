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
				Level:       schema.AgentLevel(info.Level),
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
				Level:       schema.AgentLevel(info.Level),
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
				Level:       schema.AgentLevel(info.Level),
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

func (d *Store) AllAgentWithTag() ([]*request.AgentInfo, error) {
	agentObjs, err := d.AllAgent()
	if err != nil {
		return nil, err
	}

	tagObjs, err := d.AllTags()
	if err != nil {
		return nil, err
	}
	tagMap := make(map[bson.ObjectID]string)
	for _, item := range tagObjs {
		tagMap[item.ID] = item.Name
	}

	out := make([]*request.AgentInfo, 0)
	for _, item := range agentObjs {
		tags := make([]string, 0)
		for _, id := range item.Tag {
			if name, ok := tagMap[id]; ok {
				tags = append(tags, name)
			}
		}
		out = append(out, &request.AgentInfo{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Prompt:      item.Prompt,
			Type:        int(item.Type),
			Parent:      item.Parent,
			Tag:         tags,
			Level:       int(item.Level),
		})
	}

	return out, nil
}
