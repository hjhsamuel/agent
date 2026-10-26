package tool

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

type Tag struct {
	lock sync.RWMutex

	tools  map[string]tool.Tool
	mcps   map[string]mcp.Transport
	a2as   map[string]agent.Agent
	skills map[string]struct{}
}

func (t *Tag) AddTool(name string, item tool.Tool) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.tools[name] = item

	return nil
}

func (t *Tag) AddMcp(name string, item mcp.Transport) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.mcps[name] = item

	return nil
}

func (t *Tag) AddA2A(name string, item agent.Agent) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.a2as[name] = item

	return nil
}

func (t *Tag) AddSkill(name string) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.skills[name] = struct{}{}
	return nil
}

func (t *Tag) Get() (map[string]tool.Tool, map[string]mcp.Transport, map[string]agent.Agent, map[string]struct{}) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.tools, t.mcps, t.a2as, t.skills
}

func NewTag() *Tag {
	return &Tag{
		tools:  make(map[string]tool.Tool),
		mcps:   make(map[string]mcp.Transport),
		a2as:   make(map[string]agent.Agent),
		skills: make(map[string]struct{}),
	}
}
