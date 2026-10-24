package schema

import "go.mongodb.org/mongo-driver/v2/bson"

const TagCollection = "cagent_tag"

type Tag struct {
	ID   bson.ObjectID `bson:"_id,omitempty"`
	Name string        `bson:"name"`
}
