package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hjhsamuel/agent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

var ErrSessionConflict = errors.New("session is stale or has been deleted; fetch the session again")

func adkSessionFilter(appName, userID, sessionID string) bson.D {
	return bson.D{{Key: "app_name", Value: appName}, {Key: "user_id", Value: userID}, {Key: "session_id", Value: sessionID}}
}

// withSessionTransaction does not create collections or indexes. Provision the
// collections and indexes documented in docs/mongodb-session.md before use.
func (d *Store) withSessionTransaction(ctx context.Context, fn func(context.Context) error) error {
	sess, err := d.StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)
	_, err = sess.WithTransaction(ctx, func(tx context.Context) (any, error) {
		return nil, fn(tx)
	}, options.Transaction().
		SetReadConcern(readconcern.Snapshot()).
		SetWriteConcern(writeconcern.Majority()).
		SetReadPreference(readpref.Primary()))
	return err
}

// adkStateUpdate writes only changed keys, rather than replacing a shared state
// document. Never pass raw user keys here: schema.ADKState uses encoded keys.
func adkStateUpdate(delta schema.ADKState, now time.Time) bson.D {
	set := bson.D{{Key: "updated_at", Value: now}}
	for key, value := range delta {
		set = append(set, bson.E{Key: "state." + key, Value: value})
	}
	return bson.D{{Key: "$set", Value: set}}
}

func (d *Store) readADKState(ctx context.Context, collection string, filter bson.D) (schema.ADKState, error) {
	var doc struct {
		State schema.ADKState `bson:"state"`
	}
	err := d.getCollection(collection).FindOne(ctx, filter,
		options.FindOne().SetProjection(bson.D{{Key: "state", Value: 1}, {Key: "_id", Value: 0}})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return doc.State, err
}

func (d *Store) changeADKState(ctx context.Context, collection string, filter bson.D, delta schema.ADKState, now time.Time) (schema.ADKState, error) {
	if len(delta) == 0 {
		return d.readADKState(ctx, collection, filter)
	}
	var doc struct {
		State schema.ADKState `bson:"state"`
	}
	err := d.getCollection(collection).FindOneAndUpdate(ctx, filter, adkStateUpdate(delta, now),
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After).
			SetProjection(bson.D{{Key: "state", Value: 1}, {Key: "_id", Value: 0}})).Decode(&doc)
	return doc.State, err
}

func (d *Store) CreateADKSession(ctx context.Context, doc *schema.ADKSession, appDelta, userDelta schema.ADKState) (appState, userState schema.ADKState, err error) {
	err = d.withSessionTransaction(ctx, func(tx context.Context) error {
		// Insert first: duplicate sessions fail without touching shared state.
		if _, err := d.getCollection(schema.ADKSessionCollection).InsertOne(tx, doc); err != nil {
			return fmt.Errorf("insert session: %w", err)
		}
		var err error
		appState, err = d.changeADKState(tx, schema.ADKAppStateCollection,
			bson.D{{Key: "app_name", Value: doc.AppName}}, appDelta, doc.UpdatedAt)
		if err != nil {
			return fmt.Errorf("apply app state: %w", err)
		}
		userState, err = d.changeADKState(tx, schema.ADKUserStateCollection,
			bson.D{{Key: "app_name", Value: doc.AppName}, {Key: "user_id", Value: doc.UserID}}, userDelta, doc.UpdatedAt)
		return err
	})
	return
}

func (d *Store) GetADKSession(ctx context.Context, appName, userID, sessionID string) (*schema.ADKSession, error) {
	var doc schema.ADKSession
	err := d.getCollection(schema.ADKSessionCollection).FindOne(ctx, adkSessionFilter(appName, userID, sessionID)).Decode(&doc)
	return &doc, err
}

func (d *Store) ListADKSessions(ctx context.Context, appName, userID string) ([]schema.ADKSession, error) {
	filter := bson.D{{Key: "app_name", Value: appName}}
	if userID != "" {
		filter = append(filter, bson.E{Key: "user_id", Value: userID})
	}
	cursor, err := d.getCollection(schema.ADKSessionCollection).Find(ctx, filter, options.Find().SetBatchSize(128))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []schema.ADKSession
	err = cursor.All(ctx, &docs)
	return docs, err
}

func (d *Store) GetADKAppState(ctx context.Context, appName string) (schema.ADKState, error) {
	return d.readADKState(ctx, schema.ADKAppStateCollection, bson.D{{Key: "app_name", Value: appName}})
}

// GetADKUserStates restricts the query to users present in the session result.
// Chunking bounds $in size and avoids a query per session or scanning all users.
func (d *Store) GetADKUserStates(ctx context.Context, appName string, userIDs []string) (map[string]schema.ADKState, error) {
	states := make(map[string]schema.ADKState, len(userIDs))
	for start := 0; start < len(userIDs); start += 128 {
		end := min(start+128, len(userIDs))
		filter := bson.D{{Key: "app_name", Value: appName}, {Key: "user_id", Value: bson.D{{Key: "$in", Value: userIDs[start:end]}}}}
		cursor, err := d.getCollection(schema.ADKUserStateCollection).Find(ctx, filter,
			options.Find().SetProjection(bson.D{{Key: "user_id", Value: 1}, {Key: "state", Value: 1}, {Key: "_id", Value: 0}}).SetBatchSize(128))
		if err != nil {
			return nil, err
		}
		var docs []schema.ADKUserState
		err = cursor.All(ctx, &docs)
		closeErr := cursor.Close(ctx)
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		for _, doc := range docs {
			states[doc.UserID] = doc.State
		}
	}
	return states, nil
}

func adkEventFilter(doc *schema.ADKSession, after time.Time) bson.D {
	filter := bson.D{{Key: "session", Value: doc.ID}, {Key: "sequence", Value: bson.D{{Key: "$lte", Value: doc.Version}}}}
	if !after.IsZero() {
		// BSON dates have millisecond precision. Round the inclusive lower bound
		// up so an event just before After is never accidentally included.
		bound := after.Truncate(time.Millisecond)
		if bound.Before(after) {
			bound = bound.Add(time.Millisecond)
		}
		filter = append(filter, bson.E{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: bound}}})
	}
	return filter
}

func (d *Store) GetADKSessionEvents(ctx context.Context, doc *schema.ADKSession, after time.Time, limit int) ([]schema.ADKSessionEvent, error) {
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}, {Key: "sequence", Value: -1}}).
		SetProjection(bson.D{{Key: "payload", Value: 1}, {Key: "_id", Value: 0}}).SetBatchSize(128)
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cursor, err := d.getCollection(schema.ADKSessionEventCollection).Find(ctx, adkEventFilter(doc, after), opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var events []schema.ADKSessionEvent
	err = cursor.All(ctx, &events)
	return events, err
}

func (d *Store) AppendADKSessionEvent(ctx context.Context, doc *schema.ADKSession, event *schema.ADKSessionEvent, appDelta, userDelta, sessionDelta schema.ADKState) error {
	return d.withSessionTransaction(ctx, func(tx context.Context) error {
		// Version, rather than wall-clock time, also detects concurrent writes
		// within the same millisecond. _id rejects deleted/recreated sessions.
		filter := bson.D{{Key: "_id", Value: doc.ID}, {Key: "version", Value: doc.Version}}
		update := adkStateUpdate(sessionDelta, event.Timestamp)
		update = append(update, bson.E{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}})
		result, err := d.getCollection(schema.ADKSessionCollection).UpdateOne(tx, filter, update)
		if err != nil {
			return err
		}
		if result.MatchedCount != 1 {
			return ErrSessionConflict
		}
		if _, err := d.getCollection(schema.ADKSessionEventCollection).InsertOne(tx, event); err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		// No shared-state reads or writes when the event has no such delta.
		for _, change := range []struct {
			collection string
			filter     bson.D
			delta      schema.ADKState
		}{
			{schema.ADKAppStateCollection, bson.D{{Key: "app_name", Value: doc.AppName}}, appDelta},
			{schema.ADKUserStateCollection, bson.D{{Key: "app_name", Value: doc.AppName}, {Key: "user_id", Value: doc.UserID}}, userDelta},
		} {
			if len(change.delta) == 0 {
				continue
			}
			if _, err := d.getCollection(change.collection).UpdateOne(tx, change.filter,
				adkStateUpdate(change.delta, event.Timestamp), options.UpdateOne().SetUpsert(true)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *Store) DeleteADKSession(ctx context.Context, appName, userID, sessionID string) error {
	return d.withSessionTransaction(ctx, func(tx context.Context) error {
		var doc schema.ADKSession
		err := d.getCollection(schema.ADKSessionCollection).FindOneAndDelete(tx,
			adkSessionFilter(appName, userID, sessionID),
			options.FindOneAndDelete().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&doc)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = d.getCollection(schema.ADKSessionEventCollection).DeleteMany(tx, bson.D{{Key: "session", Value: doc.ID}})
		return err
	})
}
