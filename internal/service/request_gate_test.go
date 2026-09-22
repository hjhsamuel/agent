package service

import (
	"sync"
	"testing"
)

func TestRequestGateConcurrentStop(t *testing.T) {
	for range 100 {
		g := newRequestGate()
		var wg sync.WaitGroup
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if g.acquire() {
					g.release()
				}
			}()
		}
		g.stop()
		wg.Wait()
		g.wait()
		g.stop() // Idempotent even after the last request has released.
		if g.acquire() {
			t.Fatal("stopped gate admitted request")
		}
	}
}
