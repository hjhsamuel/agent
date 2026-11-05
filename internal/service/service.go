package service

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/service/adapter"
	"github.com/hjhsamuel/agent/internal/service/ringbuffer"
	"github.com/hjhsamuel/agent/internal/store"
	"github.com/hjhsamuel/agent/internal/store/schema"
	"github.com/hjhsamuel/agent/internal/toolx/local"
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

// initLocalTools runs during construction, before API access and runner recovery.
func (s *Service) initLocalTools() error {
	tools, err := s.store.FindTools(bson.M{"type": schema.ToolLocal, "state": schema.ToolStatusOnline})
	if err != nil {
		return err
	}
	versions := make(map[string]int64, len(tools))
	for _, item := range tools {
		if _, exists := versions[item.Name]; exists {
			return fmt.Errorf("duplicate local tool %q", item.Name)
		}
		versions[item.Name] = item.Version
	}
	for _, item := range tools {
		if err := local.ValidateName(item.Name); err != nil {
			return err
		}
		if item.Version < 0 {
			return fmt.Errorf("invalid version for tool %q", item.Name)
		}
		version := item.Version
		if version == 0 {
			version, err = s.store.NextToolVersion(item.Name)
			if err != nil {
				return err
			}
		}
		if _, err := os.Lstat(local.VersionDir(item.Name, version)); os.IsNotExist(err) {
			if err := installToolVersion(item.Type, item.Name, item.Url, version); err != nil {
				return fmt.Errorf("install local tool %q: %w", item.Name, err)
			}
		} else if err != nil {
			return err
		}
		if version != item.Version {
			if err := s.store.UpdateTool(bson.M{"_id": item.ID}, bson.M{"$set": bson.M{"version": version}}); err != nil {
				return err
			}
		}
		versions[item.Name] = version
	}
	return local.Cleanup("tools", versions)
}

func (s *Service) initSkill() error {
	logrus.Infof("Start sync skills...")
	tools, err := s.store.FindTools(bson.M{"type": schema.ToolSkill})
	if err != nil {
		return err
	}

	return s.syncSkills(tools)
}

// syncSkills runs before runners start. Offline skills remain installed too.
func (s *Service) syncSkills(tools []*schema.Tool) error {
	versions := make(map[string]int64, len(tools))
	for _, item := range tools {
		if err := skillx.ValidateName(item.Name); err != nil {
			return err
		}
		if _, exists := versions[item.Name]; exists {
			return fmt.Errorf("duplicate skill %q", item.Name)
		}
		// Upgrade legacy unversioned skills before runners start.
		if item.Version == 0 {
			version, err := s.store.NextToolVersion(item.Name)
			if err != nil {
				return err
			}
			if err := installToolVersion(item.Type, item.Name, item.Url, version); err != nil {
				return err
			}
			if err := s.store.UpdateTool(bson.M{"_id": item.ID}, bson.M{"$set": bson.M{"version": version}}); err != nil {
				return err
			}
			item.Version = version
		}
		versions[item.Name] = item.Version
	}
	return local.Cleanup("skills", versions)
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

	if err = s.initLocalTools(); err != nil {
		return nil, fmt.Errorf("initialize local tools: %w", err)
	}

	// adk session storage
	s.adkSession = adapter.NewSessionService(storage)

	if err = s.initSkill(); err != nil {
		return nil, err
	}

	// ringbuffer
	s.rbs = ringbuffer.NewRingBufferManager()

	return s, nil
}
