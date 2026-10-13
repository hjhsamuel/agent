package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	ADKSessionCollection      = "adk_session"
	ADKSessionEventCollection = "adk_session_event"
	ADKAppStateCollection     = "adk_app_state"
	ADKUserStateCollection    = "adk_user_state"
)

// ADKState stores JSON values as BSON binary. Keys are prefixed with "k" and
// base64url encoded so arbitrary ADK keys can safely be used in $set paths.
// JSON preserves the same value semantics as adk-go's database service.
type ADKState map[string][]byte

// ADKSession is separate from the business Session/Message schemas.
// Version is incremented for every committed event, including state-only events.
type ADKSession struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	AppName   string        `bson:"app_name"`
	UserID    string        `bson:"user_id"`
	SessionID string        `bson:"session_id"`
	State     ADKState      `bson:"state"`
	Version   int64         `bson:"version"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

// ADKSessionEvent references a specific session document, preventing old events
// from appearing if a deleted session's public ID is reused. Payload contains
// the complete JSON event, including tool calls, binary content and metadata.
type ADKSessionEvent struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Session   bson.ObjectID `bson:"session"`
	EventID   string        `bson:"event_id"`
	Sequence  int64         `bson:"sequence"`
	Timestamp time.Time     `bson:"timestamp"`
	Payload   []byte        `bson:"payload"`
}

type ADKAppState struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	AppName   string        `bson:"app_name"`
	State     ADKState      `bson:"state"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type ADKUserState struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	AppName   string        `bson:"app_name"`
	UserID    string        `bson:"user_id"`
	State     ADKState      `bson:"state"`
	UpdatedAt time.Time     `bson:"updated_at"`
}
