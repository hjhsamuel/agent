package db

import (
	"github.com/hjhsamuel/agent/internal/db/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Dao) ListProviders(filter bson.M) ([]*schema.Provider, error) {
	collection := d.getCollection(schema.ProviderCollection)
	cursor, err := collection.Find(d.context(), filter)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Provider
	if err = cursor.All(d.context(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}
