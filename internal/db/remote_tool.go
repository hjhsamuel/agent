package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Dao) ListRemoteTools(filter bson.M) ([]*schema.RemoteTool, error) {
	collection := d.getCollection(schema.RemoteToolCollection)
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var result []*schema.RemoteTool
	err = cursor.All(context.Background(), &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
