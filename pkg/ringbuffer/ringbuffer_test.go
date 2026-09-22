package ringbuffer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReplacementWakesIdleConsumerAndReplaysHistory(t *testing.T) {
	r, _ := NewRingBuffer[string](4)
	old := r.Subscribe(0)
	r.Push("one")
	<-old.Notify()
	if _, err := old.Drain(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := old.Next(ctx); done <- err }()
	current := r.Subscribe(0)
	if err := <-done; !errors.Is(err, ErrConsumerReplaced) {
		t.Fatalf("old reader stayed alive: %v", err)
	}
	records, err := current.Drain()
	if err != nil || len(records) != 1 || records[0].Value != "one" {
		t.Fatalf("replay: %v, %v", records, err)
	}
	r.Push("two")
	select {
	case <-current.Notify():
	case <-ctx.Done():
		t.Fatal("new reader lost notification")
	}
}

func TestOverwrittenCursor(t *testing.T) {
	r, _ := NewRingBuffer[int](2)
	c := r.Subscribe(0)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	if _, err := c.Drain(); !errors.Is(err, ErrCursorExpired) {
		t.Fatalf("got %v", err)
	}
}
