package request

import "go.mongodb.org/mongo-driver/v2/bson"

type ToolInfo struct {
	ID   bson.ObjectID
	Name string
	Url  string
	Type int
	Tag  []string
}
