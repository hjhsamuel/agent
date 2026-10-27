package store

import (
	"context"
	"errors"
	"time"

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

func (d *Store) AddMessage(obj *schema.Message) (schema.SessionStatus, int64, error) {
	var state schema.SessionStatus
	// The callback may be retried after a transient transaction failure. Always
	// start from the caller's original turn rather than a previous attempt.
	original := *obj
	if original.ID.IsZero() {
		original.ID = bson.NewObjectID()
	}
	var message schema.Message
	err := d.withSessionTransaction(context.Background(), func(ctx context.Context) error {
		message = original
		if original.Turn == 0 {
			result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
				ctx,
				bson.M{
					"_id":   obj.Session,
					"state": bson.M{"$in": []schema.SessionStatus{schema.SessionDone, schema.SessionTemp}},
				},
				bson.M{
					"$inc": bson.M{"turn_seq": 1, "msg_seq": 1},
					"$set": bson.M{
						"state": schema.SessionRunning, "updated_at": time.Now(),
					},
				},
				options.FindOneAndUpdate().SetProjection(bson.M{"turn_seq": 1, "msg_seq": 1, "state": 1}),
			)
			if err := result.Err(); err != nil {
				return err
			}

			var sessionObj schema.Session
			if err := result.Decode(&sessionObj); err != nil {
				return err
			}

			state = sessionObj.State

			message.Turn = sessionObj.TurnSeq
			message.Seq = sessionObj.MsgSeq
		} else {
			result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
				ctx,
				bson.M{"_id": obj.Session},
				bson.M{"$inc": bson.M{"msg_seq": 1}},
				options.FindOneAndUpdate().SetProjection(bson.M{"msg_seq": 1, "state": 1}),
			)
			if err := result.Err(); err != nil {
				return err
			}

			var sessionObj schema.Session
			if err := result.Decode(&sessionObj); err != nil {
				return err
			}

			state = sessionObj.State

			message.Seq = sessionObj.MsgSeq
		}

		_, err := d.getCollection(schema.MessageCollection).InsertOne(ctx, &message)
		return err
	})
	if err != nil {
		return 0, 0, err
	}

	*obj = message
	return state, obj.Turn, nil
}

func (d *Store) FinishWaitInput(sessionId bson.ObjectID, messages ...*schema.Message) error {
	if len(messages) == 0 {
		return errors.New("missing confirmation messages")
	}
	ctx := context.Background()

	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
			sc,
			bson.M{"_id": sessionId, "state": schema.SessionWait},
			bson.M{
				"$set": bson.M{
					"state":      schema.SessionRunning,
					"updated_at": time.Now(),
				},
				"$inc": bson.M{"msg_seq": len(messages)},
			},
			options.FindOneAndUpdate().SetProjection(bson.M{"msg_seq": 1}),
		)
		if err = result.Err(); err != nil {
			return nil, err
		}

		var obj schema.Session
		if err = result.Decode(&obj); err != nil {
			return nil, err
		}

		for i := 0; i < len(messages); i++ {
			messages[i].Seq = obj.MsgSeq + int64(i)
		}

		_, err = d.getCollection(schema.MessageCollection).InsertMany(sc, messages)
		return nil, err
	})
	return err
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

func (d *Store) GetTurnUserMessage(sessionID bson.ObjectID, turn int64) (*schema.Message, error) {
	messages, err := d.GetMessages(
		bson.M{"session": sessionID, "turn": turn, "role": schema.RoleUser},
		options.Find().SetSort(bson.M{"seq": 1}).SetLimit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, errors.New("missing user message for running session")
	}
	return messages[0], nil
}
