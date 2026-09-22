package shard

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

type runner struct{ canceled context.Context }

func (r *runner) Start(string) error { return nil }
func (r *runner) Wait()              { <-r.canceled.Done() }

func TestRegistrationCancelAndClose(t *testing.T) {
	m := NewManager(1)
	ctx, cancel := context.WithCancel(context.Background())
	m.Set(cancel, "one", &runner{ctx})
	if !m.Cancel("one") {
		t.Fatal("missing runner")
	}
	if _, err := m.Get("one"); err != nil {
		t.Fatal("cancel removed runner before completion")
	}
	m.Delete("one")
	if m.Cancel("one") {
		t.Fatal("cancel entry retained after delete")
	}
	ctx, cancel = context.WithCancel(context.Background())
	m.Set(cancel, "two", &runner{ctx})
	m.Close()
	if _, err := m.Get("two"); err == nil {
		t.Fatal("runner retained after close")
	}
	if m.Cancel("two") {
		t.Fatal("cancel entry retained after close")
	}
}

func TestConcurrentRegistrationDoesNotReplaceRunner(t *testing.T) {
	m := NewManager(1)
	var wg sync.WaitGroup
	var registered atomic.Int32
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			if m.Set(cancel, "same", &runner{ctx}) {
				registered.Add(1)
			} else {
				cancel()
			}
		}()
	}
	wg.Wait()
	if registered.Load() != 1 {
		t.Fatalf("registered %d runners", registered.Load())
	}
	m.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if m.Set(cancel, "new", &runner{ctx}) {
		t.Fatal("registration after close succeeded")
	}
}

func TestCloseCancelsAllShardsBeforeWaiting(t *testing.T) {
	m := NewManager(2)
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	// Each runner waits for the other cancellation, so sequential cancel/wait
	// would deadlock regardless of which shard is visited first.
	m.Set(cancel1, "first", &runner{ctx2})
	m.Set(cancel2, "second", &runner{ctx1})
	m.Close()
}
