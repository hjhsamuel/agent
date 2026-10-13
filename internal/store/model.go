package store

import (
	"context"
	"math/rand/v2"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) RandomModel(ctx context.Context, kind schema.ModelKind) (*schema.Model, error) {
	modelFilter := bson.D{
		{Key: "type", Value: kind},
		{Key: "enable", Value: true},
		{Key: "api_keys", Value: bson.D{
			{
				Key: "$elemMatch",
				Value: bson.D{
					{Key: "enable", Value: true},
				},
			},
		}},
	}
	filter := bson.D{
		{Key: "enabled", Value: true},
		{Key: "models", Value: bson.D{{Key: "$elemMatch", Value: modelFilter}}},
	}
	projection := bson.D{
		{Key: "_id", Value: 0},
		{Key: "models", Value: bson.D{{Key: "$elemMatch", Value: modelFilter}}},
	}

	result := d.getCollection(schema.ProviderCollection).FindOne(
		ctx,
		filter,
		options.FindOne().SetProjection(projection),
	)

	var provider schema.Provider
	if err := result.Decode(&provider); err != nil {
		return nil, err
	}

	if len(provider.Models) == 0 || provider.Models[0] == nil {
		return nil, mongo.ErrNoDocuments
	}
	model := provider.Models[0]

	var totalWeight int64
	for _, key := range model.ApiKeys {
		if key == nil || !key.Enable {
			continue
		}
		totalWeight += key.Weight
	}
	if totalWeight == 0 {
		return nil, mongo.ErrNoDocuments
	}

	ticket := rand.Int64N(totalWeight)
	for _, key := range model.ApiKeys {
		if key == nil || !key.Enable {
			continue
		}
		if ticket < key.Weight {
			model.ApiKeys = []*schema.ApiKey{key}
			return model, nil
		}
		ticket -= key.Weight
	}
	return nil, mongo.ErrNoDocuments
}
