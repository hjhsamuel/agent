package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Dao struct {
	db string
	*mongo.Client
}

func (d *Dao) getCollection(name string) *mongo.Collection {
	return d.Database(d.db).Collection(name)
}

func NewDao(info *MongoConfig) (*Dao, error) {
	client, err := newMongoClient(info, logrus.StandardLogger())
	if err != nil {
		return nil, err
	}
	return &Dao{db: info.DB, Client: client}, nil
}

type MongoConfig struct {
	Debug       bool
	MaxIdleConn int
	MinIdleConn int
	MaxIdleTime string
	UserName    string
	Password    string
	Addr        []*MongoAddr
	DB          string
	Opts        map[string]string
}

type MongoAddr struct {
	Host string
	Port int
}

func newMongoClient(info *MongoConfig, logger *logrus.Logger) (*mongo.Client, error) {
	dsn, err := getMongoDialector(info)
	if err != nil {
		return nil, err
	}

	opts := options.Client().ApplyURI(dsn)
	if info.MaxIdleConn != 0 {
		opts.SetMaxPoolSize(uint64(info.MaxIdleConn))
	}
	if info.MinIdleConn != 0 {
		opts.SetMinPoolSize(uint64(info.MinIdleConn))
	}
	if info.MaxIdleTime != "" {
		duration, err := time.ParseDuration(info.MaxIdleTime)
		if err != nil {
			return nil, err
		}
		opts.SetMaxConnIdleTime(duration)
	}

	if logger != nil {
		sink := logrusr.New(logger).GetSink()
		logOpts := options.Logger().SetSink(sink)
		var logLevel options.LogLevel
		if info.Debug {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}

func getMongoDialector(info *MongoConfig) (string, error) {
	var b strings.Builder
	b.WriteString("mongodb://")

	// set credentials
	if info.UserName != "" {
		b.WriteString(info.UserName)
		if info.Password != "" {
			b.WriteString(":" + info.Password)
		}
		b.WriteString("@")
	}
	// set address
	addrs := make([]string, len(info.Addr))
	for i, item := range info.Addr {
		if item.Port == 0 {
			addrs[i] = item.Host
		} else {
			addrs[i] = fmt.Sprintf("%s:%d", item.Host, item.Port)
		}
	}
	if len(addrs) == 0 {
		return "", errors.New("no address")
	}
	b.WriteString(strings.Join(addrs, ","))
	b.WriteString("/")
	// set database
	if info.DB != "" {
		b.WriteString(info.DB)
	}
	// set options
	clientOptions := make([]string, 0)
	for k, v := range info.Opts {
		clientOptions = append(clientOptions, fmt.Sprintf("%s=%s", k, v))
	}
	if len(clientOptions) != 0 {
		b.WriteString("?")
		b.WriteString(strings.Join(clientOptions, ";"))
	}

	return b.String(), nil
}
