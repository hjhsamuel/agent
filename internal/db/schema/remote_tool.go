package schema

import (
	"time"

	"github.com/hjhsamuel/agent/pkg/tool"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const RemoteToolCollection = "agent_remote_tool"

type RemoteTool struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Url       string        `bson:"url"`
	Type      tool.ToolType `bson:"type"` // 只允许 mcp、a2a
	Enabled   bool          `bson:"enabled"`
	CreatedAt time.Time     `bson:"created_at"`
}
