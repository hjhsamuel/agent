package tool

import (
	"errors"
	"sync"

	"github.com/openai/openai-go/v3"
)

type Manager struct {
	lock sync.RWMutex

	toolMap map[string]Tool
}

func (m *Manager) Register(recover bool, tools ...Tool) error {
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		function := tool.Define().GetFunction()
		if function == nil {
			return errors.New("tool definition function is nil")
		}
		if function.Name == "" {
			return errors.New("tool name is empty")
		}
		toolMap[function.Name] = tool
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	for name, tool := range toolMap {
		if recover {
			m.toolMap[name] = tool
		} else {
			if _, ok := m.toolMap[name]; !ok {
				m.toolMap[name] = tool
			}
		}
	}

	return nil
}

func (m *Manager) Unregister(names ...string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, name := range names {
		delete(m.toolMap, name)
	}
}

func (m *Manager) AllDefine() []openai.ChatCompletionToolUnionParam {
	m.lock.RLock()
	defer m.lock.RUnlock()

	out := make([]openai.ChatCompletionToolUnionParam, 0, len(m.toolMap))
	for _, tool := range m.toolMap {
		out = append(out, tool.Define())
	}
	return out
}

func (m *Manager) Get(name string) (Tool, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	tool, ok := m.toolMap[name]
	if !ok {
		return nil, errors.New("tool not found")
	}
	return tool, nil
}

func NewManager() *Manager {
	return &Manager{
		toolMap: make(map[string]Tool),
	}
}
