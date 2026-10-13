package service

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
)

const CompactRatio = 0.6

type AgentConfig struct {
	Name        string
	Description string
	Prompt      string
}

type Tool struct {
	SubAgents []agent.Agent
	Tools     []tool.Tool
	Toolsets  []tool.Toolset
}
