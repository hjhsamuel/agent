package tool

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/hjhsamuel/agent/internal/tool/a2ax"
	"github.com/hjhsamuel/agent/internal/tool/mcpx"
	"github.com/hjhsamuel/agent/internal/tool/skillx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/skilltoolset"
)

type Factory struct {
	lock sync.RWMutex

	tags map[string]*Tag
}

func (f *Factory) getTags(tags ...string) []*Tag {
	if len(tags) == 0 {
		return nil
	}
	f.lock.RLock()
	defer f.lock.RUnlock()

	out := make([]*Tag, 0, len(tags))
	for _, name := range tags {
		if v, ok := f.tags[name]; ok {
			out = append(out, v)
		}
	}

	return out
}

// addTags creates only the requested categories.
func (f *Factory) addTags(tags ...string) []*Tag {
	if len(tags) == 0 {
		return nil
	}
	f.lock.Lock()
	defer f.lock.Unlock()

	items := make([]*Tag, 0, len(tags))
	for _, name := range tags {
		if f.tags[name] == nil {
			f.tags[name] = NewTag()
		}
		items = append(items, f.tags[name])
	}
	return items
}

func (f *Factory) AddMCP(name, path string, tags ...string) error {
	if len(tags) == 0 {
		return nil
	}
	transport := mcpx.NewTransport(path)

	items := f.addTags(tags...)
	for _, item := range items {
		if err := item.AddMcp(name, transport); err != nil {
			continue
		}
	}

	return nil
}

func (f *Factory) AddA2A(name, path string, tags ...string) error {
	if len(tags) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := a2ax.NewA2AFromPath(ctx, path)
	if err != nil {
		return err
	}

	items := f.addTags(tags...)
	for _, item := range items {
		if err = item.AddA2A(name, client); err != nil {
			continue
		}
	}

	return nil
}

// AddSkill registers an installed skill from skills/<name> under the given tags.
func (f *Factory) AddSkill(name string, tags ...string) error {
	for _, item := range f.addTags(tags...) {
		if err := item.AddSkill(name); err != nil {
			return err
		}
	}
	return nil
}

func (f *Factory) AddTool(name, path string, tags ...string) error {
	// TODO
	return nil
}

// All returns the union of the requested tags, deduplicated by name.
// With no tags it returns an empty pack.
func (f *Factory) All(user string, tags ...string) *Pack {
	items := f.getTags(tags...)
	if len(items) == 0 {
		return &Pack{}
	}
	tools := make(map[string]tool.Tool)
	mcps := make(map[string]mcp.Transport)
	a2as := make(map[string]agent.Agent)
	skills := make(map[string]struct{})
	for _, item := range items {
		item.lock.RLock()
		maps.Copy(tools, item.tools)
		maps.Copy(mcps, item.mcps)
		maps.Copy(a2as, item.a2as)
		maps.Copy(skills, item.skills)
		item.lock.RUnlock()
	}

	pack := &Pack{
		Tools:     make([]tool.Tool, 0, len(tools)),
		Toolsets:  make([]tool.Toolset, 0, len(mcps)+1),
		SubAgents: make([]agent.Agent, 0, len(a2as)),
	}
	for _, item := range tools {
		pack.Tools = append(pack.Tools, item)
	}
	for _, item := range a2as {
		pack.SubAgents = append(pack.SubAgents, item)
	}
	for _, transport := range mcps {
		if ts, err := mcpx.NewMCP(transport, user); err == nil {
			pack.Toolsets = append(pack.Toolsets, ts)
		}
	}
	if len(skills) > 0 {
		names := make([]string, 0, len(skills))
		for name := range skills {
			names = append(names, name)
		}
		source := skillx.NewSkillSource(names...)
		if ts, err := skilltoolset.New(context.Background(), skilltoolset.Config{Source: source}); err == nil {
			pack.Toolsets = append(pack.Toolsets, ts)
		}
	}
	return pack
}

func NewFactory() *Factory {
	return &Factory{
		tags: make(map[string]*Tag),
	}
}
