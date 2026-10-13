package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (d *Store) UpdateSession(filter, update bson.M) error {
	result, err := d.getCollection(schema.SessionCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Store) CreateTempSession(obj *schema.Session) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.SessionCollection).InsertOne(context.Background(), obj)
	if err != nil {
		return bson.ObjectID{}, err
	}
	id := result.InsertedID.(bson.ObjectID)
	return id, nil
}
