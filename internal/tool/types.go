package tool

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

type Pack struct {
	Tools     []tool.Tool
	Toolsets  []tool.Toolset
	SubAgents []agent.Agent
}
