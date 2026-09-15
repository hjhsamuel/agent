package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Dao) ListProviders(filter bson.M) ([]*schema.Provider, error) {
	collection := d.getCollection(schema.ProviderCollection)
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Provider
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}
