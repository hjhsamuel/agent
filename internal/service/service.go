package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

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
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startOnce sync.Once
	closeOnce sync.Once
	running   atomic.Bool
	requests  *requestGate

	tools     *tool.Manager
	providers *llm.Manager
	notify    *notify.Manager
	store     *db.Dao

	skills []*skill.Skill

	agents *shard.Manager // 会话agent
	events chan *notify.UpperEvent
}

func (s *Service) Start() error {
	started := false
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.ctx = ctx
		s.cancel = cancel
		s.requests = newRequestGate()
		s.notify.Start()
		s.wg.Add(1)
		go s.agentEvent()
		s.running.Store(true)
		started = true
	})
	if !started {
		return errors.New("service has already been started or closed")
	}
	return nil
}

func (s *Service) Close() {
	s.closeOnce.Do(s.close)
}

func (s *Service) close() {
	// Wait for initialization, or prevent a later Start if Close won the race.
	s.startOnce.Do(func() {})
	s.running.Store(false)
	if s.requests != nil {
		s.requests.stop()
	}
	if s.cancel != nil {
		s.cancel()
	}
	// Start may add runner goroutines, so finish admitted requests before Wait.
	if s.requests != nil {
		s.requests.wait()
	}
	if s.agents != nil {
		s.agents.Close()
	}
	if s.notify != nil {
		s.notify.Close()
	}
	s.wg.Wait()
	if s.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.store.Disconnect(ctx)
	}
}

func (s *Service) beginRequest() error {
	if !s.running.Load() || !s.requests.acquire() {
		return errors.New("service is not running")
	}
	return nil
}

func NewService(c *config.Config) (*Service, error) {
	s := &Service{
		events: make(chan *notify.UpperEvent, 1024),
	}

	if err := initStorage(c, s); err != nil {
		return nil, err
	}
	initialized := false
	defer func() {
		if !initialized {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.store.Disconnect(ctx)
		}
	}()
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

	initialized = true
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
