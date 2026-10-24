package schema

import "go.mongodb.org/mongo-driver/v2/bson"

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
	Parent      *bson.M         `bson:"parent,omitempty"`
	Tag         []bson.ObjectID `bson:"tag"`
}

type AgentType int

const (
	AgentTypeMain   AgentType = iota // main agent
	AgentTypeGroup                   // group
	AgentTypeRunner                  // runner
)
