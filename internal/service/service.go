package service

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/internal/tool"
	"github.com/hjhsamuel/agent/pkg/kms"
	"github.com/sirupsen/logrus"
	"google.golang.org/adk/v2/session"
)

type Service struct {
	store      *store.Store
	kms        *kms.KMS
	tool       *tool.Factory // 全局可用的工具、subagent
	rbs        *ringbuffer.RingBufferManager
	adkSession session.Service

	life sync.Map
	wg   sync.WaitGroup
}

func (s *Service) Start() error {
	// TODO
}

func (s *Service) Close() {
	s.life.Range(func(key, value any) bool {
		value.(context.CancelFunc)()
		return true
	})
	s.wg.Wait()

	_ = s.rbs.Close()
}

func (s *Service) setLife(id string, cancel context.CancelFunc) bool {
	_, loaded := s.life.LoadOrStore(id, cancel)
	return loaded
}

func (s *Service) lifeExists(id string) bool {
	_, ok := s.life.Load(id)
	return ok
}

func (s *Service) cancel(id string) {
	v, loaded := s.life.LoadAndDelete(id)
	if !loaded {
		return
	}

	v.(context.CancelFunc)()
}

func NewService() (*Service, error) {
	c := config.GetConfig()

	s := &Service{
		tool: tool.NewFactory(),
	}

	// kms
	secrets := kms.NewKMS()
	for _, item := range c.Secret {
		secrets.SetKey(&kms.KeySet{
			Version: int64(item.Version),
			Key:     item.Key,
		})
	}
	s.kms = secrets

	// storage
	storage, err := store.NewStore(c.Store, logrus.StandardLogger())
	if err != nil {
		return nil, err
	}
	s.store = storage

	// adk session storage
	s.adkSession = adapter.NewSessionService(storage)

	// tools
	tools, err := storage.AllTools()
	if err != nil {
		return nil, err
	}
	for _, item := range tools {
		switch item.Type {
		case schema.ToolA2A:
			_ = s.tool.AddA2A(item.Url)
		case schema.ToolMcp:
			_ = s.tool.AddMcp(item.Url)
		default:
			// TODO
			// 暂时不下载本地tool
			continue
		}
	}

	// ringbuffer
	s.rbs = ringbuffer.NewRingBufferManager()

	return s, nil
}
