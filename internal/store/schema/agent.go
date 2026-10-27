package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	RunnerName = "CAGENT"
)

const AgentCollection = "cagent_agent"

type Agent struct {
	ID          bson.ObjectID  `bson:"_id,omitempty"`
	Name        string         `bson:"name"`
	Description string         `bson:"description"`
	Prompt      string         `bson:"prompt"`
	Type        AgentType      `bson:"type"`
	Parent      *bson.ObjectID `bson:"parent,omitempty"` // 仅用于 runner 类型
	Level       AgentLevel     `bson:"level"`            // 智能程度
	CreatedAt   time.Time      `bson:"created_at"`
	UpdatedAt   time.Time      `bson:"updated_at"`
}

type AgentType int

const (
	AgentTypeMain   AgentType = iota // main agent
	AgentTypeGroup                   // group
	AgentTypeRunner                  // runner
)

type AgentLevel int

const (
	AgentLevelLow AgentLevel = iota
	AgentLevelMedium
	AgentLevelHigh
)

const (
	DefaultMainAgentName        = "CAgent"
	DefaultMainAgentDescription = "The primary agent responsible for handling user requests, coordinating tools and specialized agents, and managing conversations and complex tasks."
	DefaultMainAgentPrompt      = `You are the primary agent of a multi-agent system.

Your responsibilities:

1. Understand user requests and identify their goals and constraints.
2. Answer straightforward questions directly and accurately.
3. For complex tasks, plan the necessary steps and coordinate their execution.
4. Use available tools and delegate specialized work to appropriate sub-agents.
5. Maintain context and track progress throughout multi-step tasks.
6. Handle missing information and tool requests for user input when necessary.
7. Integrate results from tools and sub-agents into clear, useful responses.

Guidelines:

- Be accurate, efficient, and concise.
- Use tools only when they provide value.
- Avoid unnecessary steps and duplicate work.
- Never fabricate information, tool results, or task completion.
- Preserve user requirements and previously established decisions.
- Ask for clarification only when essential.
- Clearly communicate failures, blockers, and unfinished work.
- Respond in the user's language unless otherwise requested.`
)
