package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (d *Store) AllTools() ([]*schema.Tool, error) {
	cursor, err := d.getCollection(schema.ToolCollection).Find(
		context.Background(),
		bson.M{"state": schema.ToolStatusOnline},
	)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Tool
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Store) AllToolsWithTag() ([]*request.ToolInfo, error) {
	toolObjs, err := d.AllTools()
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

	out := make([]*request.ToolInfo, 0, len(toolObjs))
	for _, item := range toolObjs {
		tags := make([]string, 0)
		for _, id := range item.Tag {
			if name, ok := tagMap[id]; ok {
				tags = append(tags, name)
			}
		}
		out = append(out, &request.ToolInfo{
			ID:   item.ID,
			Name: item.Name,
			Url:  item.Url,
			Type: int(item.Type),
			Tag:  tags,
		})
	}
	return out, nil
}

func (d *Store) AddTool(obj *schema.Tool) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.ToolCollection).InsertOne(context.Background(), obj)
	if err != nil {
		return bson.ObjectID{}, err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id, nil
}

func (d *Store) UpdateTool(filter, update bson.M) error {
	result, err := d.getCollection(schema.ToolCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
