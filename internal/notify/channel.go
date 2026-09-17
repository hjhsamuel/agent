package notify

// UpperEvent
//
// agent 上报的消息
//  1. main agent 上报的 SSE 消息；
//  2. subagent 上报的 INPUT_REQUIRED
type UpperEvent struct {
	ID        string // 会话id
	IsSub     bool   // 是否为 subagent
	Event     SSEvent
	Heartbeat bool // 是否为心跳信号
}
