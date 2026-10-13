package store

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
	"sync"
)

type Store struct {
	db                  string
	sessionIndexesMu    sync.Mutex
	sessionIndexesReady bool

	*mongo.Client
}

// New uses an existing MongoDB client and database. Session writes require a
// replica set or sharded cluster, because they span multiple collections.
func New(client *mongo.Client, database string) *Store {
	return &Store{Client: client, db: database}
}

func (d *Store) getCollection(name string) *mongo.Collection {
	return d.Database(d.db).Collection(name)
}
