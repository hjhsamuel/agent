package request

import "go.mongodb.org/mongo-driver/v2/bson"

type AgentBase struct {
	Name        string
	Description string
	Prompt      string
	Level       int
}

type AgentInfo struct {
	ID          bson.ObjectID
	Name        string
	Description string
	Prompt      string
	Type        int
	Parent      *bson.ObjectID
	Tag         []string
	Level       int
}
