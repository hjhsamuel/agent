package toolx

import (
	"context"

	"github.com/hjhsamuel/agent/internal/toolx/a2ax"
	"github.com/hjhsamuel/agent/internal/toolx/mcpx"
	"github.com/hjhsamuel/agent/internal/toolx/skillx"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/auth"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/skilltoolset"
)

func GetMcp(cred auth.CredentialProvider, paths ...string) []tool.Toolset {
	out := make([]tool.Toolset, 0)
	for _, path := range paths {
		ts, err := mcpx.NewMCP(path, cred)
		if err != nil {
			continue
		}
		out = append(out, ts)
	}
	return out
}

func GetA2A(cred auth.CredentialProvider, paths ...string) []agent.Agent {
	out := make([]agent.Agent, 0)
	for _, path := range paths {
		agt, err := a2ax.NewA2A(path, cred)
		if err != nil {
			continue
		}
		out = append(out, agt)
	}
	return out
}

func GetSkills(names ...string) tool.Toolset {
	source := skillx.NewSkillSource(names...)
	if fm, err := source.ListFrontmatters(context.Background()); err != nil || len(fm) == 0 {
		return nil
	}

	sts, err := skilltoolset.New(
		context.Background(),
		skilltoolset.Config{
			Source: source,
		},
	)
	if err != nil {
		return nil
	}
	return sts
}
