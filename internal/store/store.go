package store

import (
	"github.com/bombsimon/logrusr/v4"
	"github.com/hjhsamuel/agent/config"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Store struct {
	db string

	*mongo.Client
}

func (d *Store) getCollection(name string) *mongo.Collection {
	return d.Database(d.db).Collection(name)
}

func NewStore(c *config.StoreConfig, logger *logrus.Logger) (*Store, error) {
	opts := options.Client().ApplyURI(c.Dsn)
	if c.MaxIdleConn != 0 {
		opts.SetMaxPoolSize(uint64(c.MaxIdleConn))
	}
	if c.MinIdleConn != 0 {
		opts.SetMinPoolSize(uint64(c.MinIdleConn))
	}
	if c.MaxConnIdleTime != 0 {
		opts.SetMaxConnIdleTime(c.MaxConnIdleTime)
	}

	if logger != nil {
		sink := logrusr.New(logger).GetSink()
		logOpts := options.Logger().SetSink(sink)

		var logLevel options.LogLevel
		if c.Debug {
			logLevel = options.LogLevelDebug
		} else {
			logLevel = options.LogLevelInfo
		}
		logOpts.SetComponentLevel(options.LogComponentCommand, logLevel)
		opts.SetLoggerOptions(logOpts)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	return &Store{
		db:     c.DB,
		Client: client,
	}, nil
}
