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

func (d *Store) AddMessage(obj *schema.Message) error {
	ctx := context.Background()

	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
			sc,
			bson.M{"_id": obj.Session},
			bson.M{"$inc": bson.M{"message_seq": 1}},
		)
		if err = result.Err(); err != nil {
			return nil, err
		}

		var sessionObj schema.Session
		if err = result.Decode(&sessionObj); err != nil {
			return nil, err
		}

		obj.Sequence = sessionObj.MessageSeq
		_, err = d.getCollection(schema.MessageCollection).InsertOne(sc, obj)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (d *Store) FindOneAndUpdateSession(filter, update bson.M) (*schema.Session, error) {
	result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(context.Background(), filter, update)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Session
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}
