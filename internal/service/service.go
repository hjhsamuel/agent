package service

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/db"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/internal/llm"
	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/shard"
	"github.com/hjhsamuel/agent/pkg/skill"
	"github.com/hjhsamuel/agent/pkg/tool"
	"github.com/hjhsamuel/agent/pkg/tool/a2a"
	"github.com/hjhsamuel/agent/pkg/tool/local"
	"github.com/hjhsamuel/agent/pkg/tool/mcp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	tools     *tool.Manager
	providers *llm.Manager
	notify    *notify.Manager
	store     *db.Dao

	skills []*skill.Skill

	agents *shard.Manager // 会话agent
	done   chan bson.ObjectID
	events chan *notify.UpperEvent
}

func (s *Service) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	s.notify.Start()

	s.wg.Add(1)
	go s.agentEvent()

	return nil
}

func (s *Service) Close() {
	s.cancel()

	s.notify.Close()

	s.wg.Wait()
}

func NewService(c *config.Config) (*Service, error) {
	s := &Service{
		done:   make(chan bson.ObjectID, 128),
		events: make(chan *notify.UpperEvent, 1024),
	}

	if err := initStorage(c, s); err != nil {
		return nil, err
	}
	if err := initProvider(c, s); err != nil {
		return nil, err
	}
	if err := initTools(c, s); err != nil {
		return nil, err
	}
	if err := initAgents(c, s); err != nil {
		return nil, err
	}
	if err := initNotify(c, s); err != nil {
		return nil, err
	}

	return s, nil
}

func initStorage(c *config.Config, s *Service) error {
	storage, err := db.NewDao(&db.MongoConfig{
		Debug:       c.Storage.Debug,
		MaxIdleConn: c.Storage.MaxIdleConn,
		MinIdleConn: c.Storage.MinIdleConn,
		MaxIdleTime: c.Storage.MaxIdleTime,
		DB:          c.Storage.DB,
	}, c.Storage.Dsn)
	if err != nil {
		return err
	}

	s.store = storage

	return nil
}

func initTools(c *config.Config, s *Service) error {
	m := tool.NewManager()

	// local
	for _, item := range local.GetLocalTools() {
		_ = m.Register(true, item)
	}
	// remote tools
	remoteTools, err := s.store.ListRemoteTools(bson.M{"enabled": true})
	if err != nil {
		return err
	}
	for _, item := range remoteTools {
		var tools []tool.Tool
		switch item.Type {
		case tool.A2ATool:
			tools, err = a2a.GetA2ATools(item.Url)

		case tool.McpTool:
			tools, err = mcp.GetMCPTools(item.Url)
		default:
			continue
		}
		if err != nil {
			continue
		}
		_ = m.Register(true, tools...)
	}

	s.tools = m
	return nil
}

func initProvider(c *config.Config, s *Service) error {
	m, err := llm.NewManager(c.Secret)
	if err != nil {
		return err
	}

	providers, err := s.store.ListProviders(bson.M{
		"enabled":    true,
		"type":       schema.ChatModel,
		"api_keys.0": bson.M{"$exists": true},
	})
	if err != nil {
		return err
	}
	for _, item := range providers {
		apiKeys := make([]*schema.ApiKey, 0)
		for _, key := range item.ApiKeys {
			if key.Enabled {
				apiKeys = append(apiKeys, key)
			}
		}
		if len(apiKeys) != 0 {
			m.Set(&schema.Provider{
				Name:         item.Name,
				Url:          item.Url,
				Capabilities: item.Capabilities,
				Type:         item.Type,
				ApiKeys:      apiKeys,
			})
		}
	}

	s.providers = m

	return nil
}

func initAgents(c *config.Config, s *Service) error {
	s.agents = shard.NewManager(shard.ShardCount)
	return nil
}

func initNotify(c *config.Config, s *Service) error {
	s.notify = notify.NewManager(notify.ShardCount)
	return nil
}
