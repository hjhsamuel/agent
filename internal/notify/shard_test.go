package notify

import (
	"context"
	"testing"
	"time"
)

func TestMissingBufferAndCanceledPush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newShard(ctx, 4)
	if _, ok := s.get("missing"); ok {
		t.Fatal("unexpected buffer")
	}
	if s.GetOrCreate("new") == nil {
		t.Fatal("buffer missing")
	}
	s.events = make(chan *UpperEvent)
	cancel()
	done := make(chan struct{})
	go func() { s.PushEvent("new", &DoneEvent{}); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("push blocked after cancellation")
	}
}
