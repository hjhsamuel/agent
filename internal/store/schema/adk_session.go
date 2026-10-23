package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	ADKSessionCollection      = "cagent_adk_session"
	ADKSessionEventCollection = "cagent_adk_event"
	ADKAppStateCollection     = "cagent_adk_app_state"
	ADKUserStateCollection    = "cagent_adk_user_state"
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
// from appearing if a deleted session's public ID is reused. Scalar fields and
// string lists are stored directly; complex fields are individually JSON encoded,
// following adk-go's database service.
type ADKSessionEvent struct {
	ID                      bson.ObjectID `bson:"_id,omitempty"`
	Session                 bson.ObjectID `bson:"session"`
	EventID                 string        `bson:"event_id"`
	Sequence                int64         `bson:"sequence"`
	Timestamp               time.Time     `bson:"timestamp"`
	InvocationID            string        `bson:"invocation_id"`
	Author                  string        `bson:"author"`
	Branch                  string        `bson:"branch,omitempty"`
	IsolationScope          string        `bson:"isolation_scope,omitempty"`
	LongRunningToolIDs      []string      `bson:"long_running_tool_ids,omitempty"`
	Routes                  []string      `bson:"routes,omitempty"`
	Partial                 bool          `bson:"partial,omitempty"`
	TurnComplete            bool          `bson:"turn_complete,omitempty"`
	Interrupted             bool          `bson:"interrupted,omitempty"`
	ErrorCode               string        `bson:"error_code,omitempty"`
	ErrorMessage            string        `bson:"error_message,omitempty"`
	ModelVersion            string        `bson:"model_version,omitempty"`
	SessionResumptionHandle string        `bson:"session_resumption_handle,omitempty"`
	FinishReason            string        `bson:"finish_reason,omitempty"`
	AvgLogprobs             float64       `bson:"avg_logprobs,omitempty"`

	Actions             []byte `bson:"actions,omitempty"`
	Content             []byte `bson:"content,omitempty"`
	GroundingMetadata   []byte `bson:"grounding_metadata,omitempty"`
	CustomMetadata      []byte `bson:"custom_metadata,omitempty"`
	UsageMetadata       []byte `bson:"usage_metadata,omitempty"`
	CitationMetadata    []byte `bson:"citation_metadata,omitempty"`
	LogprobsResult      []byte `bson:"logprobs_result,omitempty"`
	InputTranscription  []byte `bson:"input_transcription,omitempty"`
	OutputTranscription []byte `bson:"output_transcription,omitempty"`
	Output              []byte `bson:"output,omitempty"`
	NodeInfo            []byte `bson:"node_info,omitempty"`
	RequestedInput      []byte `bson:"requested_input,omitempty"`
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
