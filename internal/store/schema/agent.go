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
