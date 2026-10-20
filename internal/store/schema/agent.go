package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	MainAgentName = "CAGENT"
)

const AgentGroupCollection = "agent_group"

type AgentGroup struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Name        string        `bson:"name"`
	Description string        `bson:"description"`
	Prompt      string        `bson:"prompt"`
	Agents      []*Agent      `bson:"agents"`
	CreatedAt   time.Time     `bson:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at"`
}

type Agent struct {
	ID          string        `bson:"id"`
	Name        string        `bson:"name"`
	Description string        `bson:"description"`
	Prompt      string        `bson:"prompt"`
	Group       bson.ObjectID `bson:"group"`
	CreatedAt   time.Time     `bson:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at"`
}
