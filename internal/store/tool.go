package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Store) AllTools() ([]*schema.Tool, error) {
	cursor, err := d.getCollection(schema.ToolCollection).Find(
		context.Background(),
		bson.M{"enable": true},
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
