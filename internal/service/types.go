package service

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

type AgentConfig struct {
	Name        string
	Description string
	Prompt      string

	SubAgents []agent.Agent
	Tools     []tool.Tool
	Toolsets  []tool.Toolset
}
