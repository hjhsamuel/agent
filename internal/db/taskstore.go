package db

import (
	"context"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Dao) CreateRemoteTaskStore(obj *schema.RemoteTaskStore) error {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	_, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) GetRemoteTaskStore(filter bson.M) (*schema.RemoteTaskStore, error) {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	result := collection.FindOne(context.Background(), filter)

	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.RemoteTaskStore
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Dao) UpdateRemoteTaskStore(filter bson.M, update bson.M) error {
	collection := d.getCollection(schema.RemoteTaskStoreCollection)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) CreateTaskStoreServer(obj *schema.TaskStoreServer) error {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	_, err := collection.InsertOne(context.Background(), obj)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) GetTaskStoreServer(filter bson.M) (*schema.TaskStoreServer, error) {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	result := collection.FindOne(context.Background(), filter)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.TaskStoreServer
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Dao) UpdateTaskStoreServer(filter bson.M, update bson.M) error {
	collection := d.getCollection(schema.TaskStoreServerCollection)
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	return nil
}
