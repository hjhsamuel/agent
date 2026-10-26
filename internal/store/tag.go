package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Store) AllTags() ([]*schema.Tag, error) {
	cursor, err := d.getCollection(schema.TagCollection).Find(
		context.Background(),
		bson.M{},
	)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Tag
	if err = cursor.All(context.Background(), &objs); err != nil {
		return nil, err
	}

	return objs, nil
}
