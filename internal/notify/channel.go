package notify

// ChannelEvent agent 上报给 SSE 的消息
type ChannelEvent struct {
	ID    string
	Event SSEvent
}
