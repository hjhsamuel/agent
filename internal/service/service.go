package service

import (
	"os"
	"sync"
	"sync/atomic"

	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/pkg/kms"
	"github.com/hjhsamuel/agent/pkg/skillx"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/adk/v2/session"
)

type Service struct {
	store      *store.Store
	kms        *kms.KMS
	rbs        *ringbuffer.RingBufferManager
	adkSession session.Service

	life        sync.Map
	wg          sync.WaitGroup
	lifecycleMu sync.Mutex
	stopping    atomic.Bool
}

func (s *Service) initSkill() error {
	tools, err := s.store.AllTools()
	if err != nil {
		return err
	}

	_ = os.MkdirAll("skills", os.ModePerm)

	for _, item := range tools {
		if item.Type != schema.ToolSkill {
			continue
		}
		if skillx.IsSkillExist(item.Name) {
			continue
		}
		_ = s.downloadAndExtractTool(item.Type, item.Url, item.Name)
	}
	return nil
}

func (s *Service) Start() error {
	// 恢复任务
	objs, err := s.store.FindSession(bson.M{
		"state": bson.M{
			"$in": []schema.SessionStatus{
				schema.SessionRunning,
				schema.SessionInputReady,
			},
		},
	})
	if err != nil {
		return err
	}

	for _, item := range objs {
		if err := s.recoverSession(item); err != nil {
			logrus.WithError(err).WithField("session", item.ID.Hex()).Error("recover session")
		}
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

	s := &Service{}

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

	if err = s.initSkill(); err != nil {
		return nil, err
	}

	// ringbuffer
	s.rbs = ringbuffer.NewRingBufferManager()

	return s, nil
}
