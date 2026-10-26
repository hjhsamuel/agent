package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/internal/tool"
	"github.com/hjhsamuel/agent/pkg/kms"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/session"
)

type Service struct {
	store      *store.Store
	kms        *kms.KMS
	tool       *tool.Factory // 全局可用的工具、subagent、skills
	rbs        *ringbuffer.RingBufferManager
	adkSession session.Service

	life        sync.Map
	wg          sync.WaitGroup
	lifecycleMu sync.Mutex
	stopping    atomic.Bool
}

type sessionLife struct {
	context.Context
	cancel     context.CancelFunc
	finishOnce sync.Once
}

func (s *Service) Start() error {
	// TODO
	// 检查 skills

	// 恢复任务
	objs, err := s.store.FindSession(bson.M{
		"state": bson.M{
			"$in": []schema.SessionStatus{
				schema.SessionRunning,
				schema.SessionWait,
			},
		},
	})
	if err != nil {
		return err
	}

	for _, item := range objs {
		_ = s.recoverSession(item)
	}

	return nil
}

func (s *Service) Close() {
	s.lifecycleMu.Lock()
	s.stopping.Store(true)
	s.life.Range(func(key, value any) bool {
		value.(*sessionLife).cancel()
		return true
	})
	s.lifecycleMu.Unlock()
	s.wg.Wait()

	_ = s.rbs.Close()
}

func (s *Service) setLife(id string, life *sessionLife) bool {
	_, loaded := s.life.LoadOrStore(id, life)
	return loaded
}

func (s *Service) lifeExists(id string) bool {
	_, ok := s.life.Load(id)
	return ok
}

// releaseLife permits the next turn without closing this turn's SSE producer.
func (s *Service) releaseLife(id string, life *sessionLife) {
	s.life.CompareAndDelete(id, life)
}

// finishLife only cleans up the owning turn, even if its successor has started.
func (s *Service) finishLife(id string, life *sessionLife) {
	life.finishOnce.Do(func() {
		s.releaseLife(id, life)
		life.cancel()
		s.wg.Done()
	})
}

func NewService() (*Service, error) {
	c := config.GetConfig()

	s := &Service{
		tool: tool.NewFactory(),
	}

	// kms
	secrets := kms.NewKMS()
	for _, item := range c.Secret {
		err := secrets.SetKey(&kms.KeySet{
			Version: int64(item.Version),
			Key:     item.Key,
		})
		if err != nil {
			return nil, err
		}
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
	tools, err := storage.AllToolsWithTag()
	if err != nil {
		return nil, err
	}
	for _, item := range tools {
		switch schema.ToolType(item.Type) {
		case schema.ToolA2A:
			_ = s.tool.AddA2A(item.Name, item.Url, item.Tag...)
		case schema.ToolMcp:
			_ = s.tool.AddMCP(item.Name, item.Url, item.Tag...)
		case schema.ToolSkill:
			_ = s.tool.AddSkill(item.Name, item.Tag...)
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
