package notify

import (
	"github.com/gin-contrib/sse"
)

type SSEvent interface {
	Event() sse.Event
}

type EventType string

const (
	InitEventType          EventType = "init"           // 创建会话
	MetaEventType          EventType = "meta"           // 会话信息
	CompletionEventType    EventType = "completion"     // 正文内容
	ReasoningEventType     EventType = "reasoning"      // 思考内容
	DoneEventType          EventType = "done"           // 完整输出信号
	ResetEventType         EventType = "reset"          // 流输出中断，重置信号
	ErrorEventType         EventType = "error"          // 错误
	InputRequiredEventType EventType = "input_required" // 等待用户输入
	ApprovedEventType      EventType = "approved"       // 等待人工审批
	ExpiredEventType       EventType = "expired"        // sse 重连导致消息不全
)

type InitEvent struct {
	ID string
}

func (e *InitEvent) Event() sse.Event {
	return sse.Event{
		Event: string(InitEventType),
		Data:  map[string]any{"id": e.ID},
	}
}

type MetaEvent struct {
	Title string
}

func (e *MetaEvent) Event() sse.Event {
	return sse.Event{
		Event: string(MetaEventType),
		Data:  map[string]any{"title": e.Title},
	}
}

type CompletionEvent struct {
	Content string
}

func (e *CompletionEvent) Event() sse.Event {
	return sse.Event{
		Event: string(CompletionEventType),
		Data:  map[string]any{"content": e.Content},
	}
}

type ReasoningEvent struct {
	Content string
}

func (e *ReasoningEvent) Event() sse.Event {
	return sse.Event{
		Event: string(ReasoningEventType),
		Data:  map[string]any{"content": e.Content},
	}
}

type DoneEvent struct{}

func (e *DoneEvent) Event() sse.Event {
	return sse.Event{
		Event: string(DoneEventType),
	}
}

type ResetEvent struct {
	Error string
}

func (e *ResetEvent) Event() sse.Event {
	return sse.Event{
		Event: string(ResetEventType),
		Data:  map[string]any{"error": e.Error},
	}
}

type ErrorEvent struct {
	Error string
}

func (e *ErrorEvent) Event() sse.Event {
	return sse.Event{
		Event: string(ErrorEventType),
		Data:  map[string]any{"error": e.Error},
	}
}

type InputRequiredEvent struct {
	Events []*InputRequiredItem
}

type InputRequiredItem struct {
	ContextId string
	TaskId    string
	Content   string
}

func (e *InputRequiredEvent) Event() sse.Event {
	events := make([]any, 0, len(e.Events))
	for _, event := range e.Events {
		events = append(events, map[string]any{
			"context_id": event.ContextId,
			"task_id":    event.TaskId,
			"content":    event.Content,
		})
	}
	return sse.Event{
		Event: string(InputRequiredEventType),
		Data:  map[string]any{"events": events},
	}
}

type ApprovedEvent struct {
	ContextId   string
	TaskId      string
	Content     string
	Description string
}

func (e *ApprovedEvent) Event() sse.Event {
	return sse.Event{
		Event: string(ApprovedEventType),
		Data: map[string]any{
			"context_id":  e.ContextId,
			"task_id":     e.TaskId,
			"content":     e.Content,
			"description": e.Description,
		},
	}
}

type ExpiredEvent struct {
	Oldest uint64
	Latest uint64
}

func (e *ExpiredEvent) Event() sse.Event {
	return sse.Event{
		Event: string(ExpiredEventType),
		Data: map[string]any{
			"oldest": e.Oldest,
			"latest": e.Latest,
		},
	}
}
