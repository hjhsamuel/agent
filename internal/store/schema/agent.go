package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	MainAgentName = "cagent"
	RunnerName    = "CAGENT"
)

type Agent struct {
	ID          bson.M          `bson:"_id,omitempty"`
	Name        string          `bson:"name"`
	Description string          `bson:"description"`
	Prompt      string          `bson:"prompt"`
	Type        AgentType       `bson:"type"`
	Parent      *bson.M         `bson:"parent,omitempty"` // 仅用于 runner 类型
	Tag         []bson.ObjectID `bson:"tag"`
	CreatedAt   time.Time       `bson:"created_at"`
	UpdatedAt   time.Time       `bson:"updated_at"`
}

type AgentType int

const (
	AgentTypeMain   AgentType = iota // main agent
	AgentTypeGroup                   // group
	AgentTypeRunner                  // runner
)
