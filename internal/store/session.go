package store

import (
	"context"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) FindSession(
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Session, error) {
	ctx := context.Background()
	cursor, err := d.getCollection(schema.SessionCollection).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Session
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}
	return objs, nil
}

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

func (d *Store) FindOneAndUpdateSession(
	filter bson.M,
	update bson.M,
	opts ...options.Lister[options.FindOneAndUpdateOptions],
) (*schema.Session, error) {
	result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(context.Background(), filter, update, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Session
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Store) DeleteSession(user string, id bson.ObjectID) error {
	// session
	result, err := d.getCollection(schema.SessionCollection).UpdateOne(
		context.Background(),
		bson.M{"_id": id, "user": user, "state": schema.SessionDone},
		bson.M{"$set": bson.M{"state": schema.SessionDeleted}},
	)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (d *Store) GetMessages(
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Message, error) {
	ctx := context.Background()

	cursor, err := d.getCollection(schema.MessageCollection).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Message
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}

	return objs, nil
}
