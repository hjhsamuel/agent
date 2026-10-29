package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func (d *Store) CreateNewTurn(obj *schema.Message) (schema.SessionStatus, int64, error) {
	var state schema.SessionStatus
	err := d.withSessionTransaction(context.Background(), func(ctx context.Context) error {
		result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
			ctx,
			bson.M{
				"_id":   obj.Session,
				"state": bson.M{"$in": []schema.SessionStatus{schema.SessionTemp, schema.SessionDone}},
			},
			bson.M{
				"$inc": bson.M{"turn_seq": 1, "msg_seq": 1},
				"$set": bson.M{
					"state":      schema.SessionInputReady,
					"updated_at": time.Now(),
				},
			},
		)
		if err := result.Err(); err != nil {
			return err
		}

		var sessionObj schema.Session
		if err := result.Decode(&sessionObj); err != nil {
			return err
		}

		state = sessionObj.State

		obj.Turn = sessionObj.TurnSeq
		obj.Seq = sessionObj.MsgSeq

		_, err := d.getCollection(schema.MessageCollection).InsertOne(ctx, obj)
		return err
	})
	if err != nil {
		return 0, 0, err
	}

	return state, obj.Turn, nil
}

func (d *Store) AddMessage(obj *schema.Message) error {
	return d.withSessionTransaction(context.Background(), func(ctx context.Context) error {
		result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
			ctx,
			bson.M{"_id": obj.Session, "state": schema.SessionRunning},
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

		obj.Seq = sessionObj.MsgSeq

		_, err := d.getCollection(schema.MessageCollection).InsertOne(ctx, &obj)
		return err
	})
}

// SubmitWaitInput atomically stores an answer and makes the complete input ready.
func (d *Store) SubmitWaitInput(user string, id bson.ObjectID, callID, content string) (*schema.Session, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("input is empty")
	}

	var obj schema.Session
	err := d.withSessionTransaction(context.Background(), func(ctx context.Context) error {
		now := time.Now()
		result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
			ctx,
			bson.M{
				"_id":   id,
				"user":  user,
				"state": schema.SessionWait,
				"wait":  bson.M{"$elemMatch": bson.M{"id": callID, "content": ""}}},
			bson.M{
				"$set": bson.M{
					"wait.$.content":    content,
					"wait.$.updated_at": now,
					"updated_at":        now,
				},
			},
			options.FindOneAndUpdate().SetReturnDocument(options.After))
		if err := result.Decode(&obj); err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return err
			}
			// A previous submission may have committed before runner startup
			// failed. Reuse that input without inserting its messages again.
			// Match both owner and content so retries cannot replace an answer.
			return d.getCollection(schema.SessionCollection).FindOne(
				ctx,
				bson.M{
					"_id":   id,
					"user":  user,
					"state": schema.SessionInputReady,
					"wait":  bson.M{"$elemMatch": bson.M{"id": callID}},
				},
			).Decode(&obj)
		}

		for _, input := range obj.Wait {
			if input == nil || input.ID == "" || input.Content == "" {
				return nil
			}
		}
		if len(obj.Wait) == 0 {
			return errors.New("missing confirmation inputs")
		}

		messages := make([]*schema.Message, 0, len(obj.Wait))
		for _, input := range obj.Wait {
			messages = append(messages, &schema.Message{
				Session:   id,
				Turn:      obj.TurnSeq - 1,
				Role:      schema.RoleUser,
				Content:   fmt.Sprintf("> %s \n\n%s", input.Hint, input.Content),
				CreatedAt: input.UpdatedAt,
			})
		}
		if err := d.finishWaitInput(ctx, id, messages...); err != nil {
			return err
		}
		obj.State = schema.SessionInputReady
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func (d *Store) finishWaitInput(ctx context.Context, id bson.ObjectID, messages ...*schema.Message) error {
	if len(messages) == 0 {
		return errors.New("missing confirmation messages")
	}

	result := d.getCollection(schema.SessionCollection).FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":   id,
			"state": schema.SessionWait,
		},
		bson.M{
			"$set": bson.M{"state": schema.SessionInputReady, "updated_at": time.Now()},
			"$inc": bson.M{"msg_seq": len(messages)},
		},
		options.FindOneAndUpdate().SetProjection(bson.M{"msg_seq": 1}),
	)

	var obj schema.Session
	if err := result.Decode(&obj); err != nil {
		return err
	}
	for i, message := range messages {
		message.Seq = obj.MsgSeq + int64(i)
	}
	_, err := d.getCollection(schema.MessageCollection).InsertMany(ctx, messages)
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
