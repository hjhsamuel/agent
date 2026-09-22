package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatContentAndStreamUsage(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream  bool `json:"stream"`
			Options struct {
				Include bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Stream {
			if !body.Options.Include {
				t.Error("stream usage not requested")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\n")
			io.WriteString(w, "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\ndata: [DONE]\n\n")
		} else {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"hello"}}]}`)
		}
	}))
	defer s.Close()
	p, _ := NewOpenAI(s.URL, "test")
	message, err := p.Chat(context.Background(), "test", "", nil, nil)
	if err != nil || message.Content != "hello" {
		t.Fatalf("chat: %+v, %v", message, err)
	}
	message, err = p.Stream(context.Background(), "test", "", nil, nil, func(*StreamChunk, error) error { return nil })
	if err != nil || message.Content != "hello" || message.Usage.Total != 12 {
		t.Fatalf("stream: %+v, %v", message, err)
	}
	stop := errors.New("consumer stopped")
	_, err = p.Stream(context.Background(), "test", "", nil, nil, func(*StreamChunk, error) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("callback error ignored: %v", err)
	}
}

func TestToolHistorySerializesFunctionCall(t *testing.T) {
	p := &OpenAI{}
	params := p.buildParams("test", "system", []*Message{
		{Role: RoleAssistant, ToolCalls: []*ToolCall{{ID: "call-1", Name: "echo", Arguments: `{"x":1}`}}},
		{Role: RoleTool, ToolCallId: "call-1", Content: "result"},
	}, nil)
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Messages []struct {
			Role  string `json:"role"`
			Calls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Messages) != 3 || len(body.Messages[1].Calls) != 1 {
		t.Fatalf("tool call missing: %s", raw)
	}
	call := body.Messages[1].Calls[0]
	if call.ID != "call-1" || call.Type != "function" || call.Function.Name != "echo" || call.Function.Arguments != `{"x":1}` || body.Messages[2].ToolCallID != call.ID {
		t.Fatalf("invalid history: %s", raw)
	}
}
