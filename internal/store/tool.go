package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) FindTools(
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Tool, error) {
	ctx := context.Background()

	cursor, err := d.getCollection(schema.ToolCollection).Find(
		ctx,
		filter,
		opts...,
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Tool
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Store) AddTool(obj *schema.Tool) (bson.ObjectID, error) {
	if obj.Type == schema.ToolLocal {
		_, err := d.getCollection(schema.ToolCollection).Indexes().CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetName("unique_local_tool_name").SetUnique(true).SetPartialFilterExpression(bson.M{"type": schema.ToolLocal}),
		})
		if err != nil {
			return bson.ObjectID{}, err
		}
	}
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

func (d *Store) FindOneAndUpdateTool(
	filter bson.M,
	update bson.M,
	opts ...options.Lister[options.FindOneAndUpdateOptions],
) (*schema.Tool, error) {
	result := d.getCollection(schema.ToolCollection).FindOneAndUpdate(context.Background(), filter, update, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Tool
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Store) DeleteTool(filter bson.M) error {
	_, err := d.getCollection(schema.ToolCollection).DeleteOne(context.Background(), filter)
	return err
}

// NextToolVersion reserves a monotonically increasing version per Name.
// Counters survive deletion so a recreated tool cannot overwrite runners' files.
func (d *Store) NextToolVersion(name string) (int64, error) {
	var counter struct {
		Version int64 `bson:"version"`
	}
	err := d.getCollection("cagent_tool_version").FindOneAndUpdate(context.Background(),
		bson.M{"_id": name}, bson.M{"$inc": bson.M{"version": int64(1)}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&counter)
	return counter.Version, err
}
