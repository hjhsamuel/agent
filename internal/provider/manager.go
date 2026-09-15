package provider

import (
	"errors"
	"sync"

	"github.com/hjhsamuel/agent/internal/db/schema"
	"github.com/hjhsamuel/agent/pkg/provider"
)

type Manager struct {
	lock sync.RWMutex

	providers    map[string]*LLM
	defaultModel string
}

func (m *Manager) Get(model string) (*LLM, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if v, ok := m.providers[model]; ok {
		return v, nil
	}

	return nil, errors.New("provider not found")
}

func (m *Manager) Set(
	model string,
	client provider.Provider,
	capabilities *schema.ModelCapabilities,
	isDefault bool,
) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.providers[model] = &LLM{
		Model:        model,
		Capabilities: capabilities,
		provider:     client,
	}
	if isDefault {
		m.defaultModel = model
	}

	return nil
}

func (m *Manager) Default() *LLM {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.providers[m.defaultModel]
}

func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]*LLM),
	}
}
