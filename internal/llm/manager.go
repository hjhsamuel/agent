package llm

import (
	"errors"
	"math/rand/v2"
	"sync"

	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/kms"
	"github.com/hjhsamuel/agent/pkg/provider"
)

type Manager struct {
	lock   sync.RWMutex
	models map[schema.ModelType]*schema.Provider

	keys map[int]string
}

func (m *Manager) Get(t schema.ModelType) (*LLM, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	model, ok := m.models[t]
	if !ok {
		return nil, errors.New("not found")
	}

	if len(model.ApiKeys) == 0 {
		return nil, errors.New("no available api keys")
	}

	var (
		out string
		err error
	)
	start := rand.IntN(len(model.ApiKeys))
	for i := 0; i < len(model.ApiKeys); i++ {
		index := start + i
		if index >= len(model.ApiKeys) {
			index -= len(model.ApiKeys)
		}

		apiKey := model.ApiKeys[index]
		key, ok := m.keys[apiKey.Version]
		if !ok {
			continue
		}
		out, err = kms.Decrypt([]byte(key), apiKey.Ciphertext, apiKey.Nonce)
		if err != nil {
			continue
		}
	}

	if out == "" {
		return nil, errors.New("no available api keys")
	}

	client, err := provider.NewOpenAI(model.Url, out)
	if err != nil {
		return nil, err
	}

	return &LLM{
		Model:        model.Name,
		Capabilities: model.Capabilities,
		provider:     client,
	}, nil
}

func (m *Manager) Set(info *schema.Provider) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.models[info.Type] = info
}

func NewManager(secrets []*config.SecretConfig) (*Manager, error) {
	if len(secrets) == 0 {
		return nil, errors.New("no secret set")
	}

	keys := make(map[int]string)
	for _, item := range secrets {
		if len(item.Key) != kms.KeyLength {
			continue
		}
		keys[item.Version] = item.Key
	}
	if len(keys) == 0 {
		return nil, errors.New("no available keys")
	}

	return &Manager{
		models: make(map[schema.ModelType]*schema.Provider),
		keys:   keys,
	}, nil
}
