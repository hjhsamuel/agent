package tool

import (
	"context"
	"sync"

	"github.com/hjhsamuel/agent/internal/tool/a2ax"
	"github.com/hjhsamuel/agent/internal/tool/local"
	"github.com/hjhsamuel/agent/internal/tool/mcpx"
	"github.com/hjhsamuel/agent/internal/tool/skillx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/skilltoolset"
	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

type Factory struct {
	lock sync.RWMutex

	localTools   []tool.Tool
	mcpTransport []mcp.Transport
	a2aAgent     []agent.Agent
	skillSource  skill.Source
}

func (f *Factory) init() {
	// local tool
	f.localTools = local.GetTools()

	f.skillSource = skillx.NewSkillSource()
}

func (f *Factory) AddMcp(path string) error {
	transport := mcpx.NewTransport(path)

	f.lock.Lock()
	defer f.lock.Unlock()

	f.mcpTransport = append(f.mcpTransport, transport)
	return nil
}

func (f *Factory) AddA2A(path string) error {
	card, err := a2ax.ResolveCard(context.Background(), path)
	if err != nil {
		return err
	}
	agt, err := a2ax.NewA2A(card)
	if err != nil {
		return err
	}

	f.lock.Lock()
	defer f.lock.Unlock()

	f.a2aAgent = append(f.a2aAgent, agt)
	return nil
}

func (f *Factory) All(user string) *Pack {
	f.lock.RLock()
	defer f.lock.RUnlock()

	pack := Pack{
		Tools:     f.localTools,
		SubAgents: f.a2aAgent,
		Toolsets:  make([]tool.Toolset, 0),
	}

	for _, item := range f.mcpTransport {
		ts, err := mcpx.NewMCP(item, user)
		if err != nil {
			continue
		}
		pack.Toolsets = append(pack.Toolsets, ts)
	}

	if f.skillSource != nil {
		skillToolset, err := skilltoolset.New(context.Background(), skilltoolset.Config{Source: f.skillSource})
		if err == nil {
			pack.Toolsets = append(pack.Toolsets, skillToolset)
		}
	}

	return &pack
}

func NewFactory() *Factory {
	f := &Factory{
		localTools:   make([]tool.Tool, 0),
		mcpTransport: make([]mcp.Transport, 0),
		a2aAgent:     make([]agent.Agent, 0),
	}
	f.init()
	return f
}

type Pack struct {
	Tools     []tool.Tool
	Toolsets  []tool.Toolset
	SubAgents []agent.Agent
}
