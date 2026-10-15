package schema

import (
	"time"

	"github.com/openai/openai-go/v3/responses"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const SessionCollection = "session"

type Session struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	User       string        `bson:"user"`
	Name       string        `bson:"name"`
	State      SessionStatus `bson:"state"`
	MessageSeq int64         `bson:"message_seq"`
	CreatedAt  time.Time     `bson:"created_at"`
	UpdatedAt  time.Time     `bson:"updated_at"`
	Wait       []*WaitInput  `bson:"wait"` // input_required 信息
}

type WaitInput struct {
	ID      string `bson:"id"`
	Hint    string `bson:"hint"`    // 传递给用户的提示信息
	Content string `bson:"content"` // 用户输入的补充信息
}

type SessionStatus int

const (
	SessionTemp    SessionStatus = iota // 默认状态，临时会话
	SessionRunning                      // 运行中
	SessionDone                         // 结束运行
	SessionWait                         // 等待用户输入
)

const MessageCollection = "message"

type Message struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Session   bson.ObjectID `bson:"session"`
	Sequence  int64         `bson:"sequence"`
	Role      Role          `bson:"role"`
	Content   string        `bson:"content"`
	CreatedAt time.Time     `bson:"created_at"`
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Part struct {
	Content    []byte `bson:"content"`
	ToolCallID string `bson:"tool_call_id"`
	ToolName   string `bson:"tool_name"`
}

type Usage struct {
	InputTokens         int64 `bson:"input_tokens"`
	OutputTokens        int64 `bson:"output_tokens"`
	TotalTokens         int64 `bson:"total_tokens"`
	CachedTokens        int64 `bson:"cached_tokens"`
	CacheWriteTokens    int64 `bson:"cache_write_tokens"`
	ReasoningTokens     int64 `bson:"reasoning_tokens"`
	OrdinaryInputTokens int64 `bson:"ordinary_input_tokens"`
}

// CacheTokenHitRate 缓存命中率
func (u Usage) CacheTokenHitRate() float64 {
	if u.InputTokens == 0 {
		return 0
	}

	return float64(u.CachedTokens) / float64(u.InputTokens)
}

// CacheWriteRate 输入中有多少比例在建立缓存
func (u Usage) CacheWriteRate() float64 {
	if u.InputTokens == 0 {
		return 0
	}

	return float64(u.CacheWriteTokens) / float64(u.InputTokens)
}

func (u Usage) Add(usage responses.ResponseUsage) {
	var (
		input  = usage.InputTokens
		output = usage.OutputTokens
		total  = usage.TotalTokens

		cached     = usage.InputTokensDetails.CachedTokens
		cacheWrite = usage.InputTokensDetails.CacheWriteTokens
		reasoning  = usage.OutputTokensDetails.ReasoningTokens
	)

	// input_tokens = ordinary_input + cached_input + cached_write_input
	ordinary := input - cached - cacheWrite
	if ordinary < 0 {
		ordinary = 0
	}

	u.InputTokens += input
	u.OutputTokens += output
	u.TotalTokens += total

	u.CachedTokens += cached
	u.CacheWriteTokens += cacheWrite
	u.OrdinaryInputTokens += ordinary

	u.ReasoningTokens += reasoning
}
