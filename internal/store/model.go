package store

import (
	"context"
	"math/rand/v2"

	"github.com/hjhsamuel/agent/internal/store/request"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) RandomModel(ctx context.Context, kind schema.ModelKind, levels ...schema.ModelLevel) (*schema.Model, error) {
	filter := bson.D{
		{Key: "type", Value: kind},
		{Key: "enable", Value: true},
		{Key: "api_keys", Value: bson.D{
			{
				Key: "$elemMatch",
				Value: bson.D{
					{Key: "enable", Value: true},
					{Key: "weight", Value: bson.M{"$gt": 0}},
				},
			},
		}},
	}
	opts := options.FindOne()
	if len(levels) != 0 {
		filter = append(filter, bson.E{Key: "level", Value: bson.M{"$in": levels}})
	} else {
		// Select the lowest available intelligence level in a single query.
		opts.SetSort(bson.D{{Key: "level", Value: 1}})
	}

	result := d.getCollection(schema.ModelCollection).FindOne(
		ctx,
		filter,
		opts,
	)

	var model schema.Model
	if err := result.Decode(&model); err != nil {
		return nil, err
	}

	var totalWeight int64
	for _, key := range model.ApiKeys {
		if key == nil || !key.Enable || key.Weight <= 0 {
			continue
		}
		totalWeight += key.Weight
	}
	if totalWeight == 0 {
		return nil, mongo.ErrNoDocuments
	}

	ticket := rand.Int64N(totalWeight)
	for _, key := range model.ApiKeys {
		if key == nil || !key.Enable || key.Weight <= 0 {
			continue
		}
		if ticket < key.Weight {
			model.ApiKeys = []*schema.ApiKey{key}
			return &model, nil
		}
		ticket -= key.Weight
	}
	return nil, mongo.ErrNoDocuments
}

func (d *Store) AddModel(modelObj *schema.Model) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.ModelCollection).InsertOne(context.Background(), modelObj)
	if err != nil {
		return bson.ObjectID{}, err
	}
	id := result.InsertedID.(bson.ObjectID)

	return id, nil
}

func (d *Store) GroupModel() ([]*request.GroupModel, error) {
	ctx := context.Background()
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "name", Value: 1},
			{Key: "_id", Value: 1},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$provider"},
			{Key: "models", Value: bson.D{{Key: "$push", Value: bson.D{
				{Key: "id", Value: "$_id"},
				{Key: "name", Value: "$name"},
				{Key: "type", Value: "$type"},
				{Key: "enable", Value: "$enable"},
				{Key: "level", Value: "$level"},
			}}}},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "provider", Value: "$_id"},
			{Key: "models", Value: 1},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "provider", Value: 1}}}},
	}
	cursor, err := d.getCollection(schema.ModelCollection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	groups := make([]*request.GroupModel, 0)
	if err = cursor.All(ctx, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func (d *Store) AddApiKey(id bson.ObjectID, obj *schema.ApiKey) (int64, error) {
	nextID := bson.D{{Key: "$ifNull", Value: bson.A{"$seq", int64(1)}}}
	update := mongo.Pipeline{
		bson.D{
			{
				Key: "$set",
				Value: bson.D{
					{
						Key:   "seq",
						Value: bson.D{{Key: "$add", Value: bson.A{nextID, int64(1)}}},
					},
					{
						Key: "api_keys",
						Value: bson.D{
							{
								Key: "$concatArrays",
								Value: bson.A{
									bson.D{{Key: "$ifNull", Value: bson.A{"$api_keys", bson.A{}}}},
									bson.A{bson.D{
										{
											Key: "$mergeObjects",
											Value: bson.A{
												bson.D{{Key: "$literal", Value: obj}},
												bson.D{{Key: "id", Value: nextID}},
											},
										}},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	result := d.getCollection(schema.ModelCollection).FindOneAndUpdate(
		context.Background(),
		bson.D{{Key: "_id", Value: id}},
		update,
		options.FindOneAndUpdate().SetProjection(bson.M{"seq": 1}),
	)
	var modelObj schema.Model
	if err := result.Decode(&modelObj); err != nil {
		return 0, err
	}

	return modelObj.Seq, nil
}

func (d *Store) GetModel(
	filter bson.M,
	opts ...options.Lister[options.FindOneOptions],
) (*schema.Model, error) {
	result := d.getCollection(schema.ModelCollection).FindOne(context.Background(), filter, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Model
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Store) UpdateModel(filter, update bson.M) error {
	_, err := d.getCollection(schema.ModelCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (d *Store) DeleteApiKey(filter, update bson.M) error {
	_, err := d.getCollection(schema.ModelCollection).UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}

	return nil
}
