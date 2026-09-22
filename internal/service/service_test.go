package service

import (
	"sync"
	"testing"
	"time"

	"github.com/hjhsamuel/agent/internal/notify"
	"github.com/hjhsamuel/agent/internal/service/shard"
)

func testService() *Service {
	return &Service{notify: notify.NewManager(1), agents: shard.NewManager(1), events: make(chan *notify.UpperEvent, 1)}
}

func TestCloseTerminatesEventLoop(t *testing.T) {
	s := testService()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown loop did not exit")
	}
}

func TestRequestsOverlapAndCloseWaitsForAdmittedRequests(t *testing.T) {
	s := testService()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.beginRequest(); err != nil {
		t.Fatal(err)
	}
	second := make(chan error, 1)
	go func() {
		err := s.beginRequest()
		if err == nil {
			s.requests.release()
		}
		second <- err
	}()
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("requests were serialized")
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-s.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("close did not cancel execution")
	}
	if err := s.beginRequest(); err == nil {
		s.requests.release()
		t.Error("request admitted after shutdown")
	}
	select {
	case <-done:
		t.Error("close returned before admitted request completed")
	default:
	}
	s.requests.release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}
}

func TestConcurrentStartAndClose(t *testing.T) {
	for range 50 {
		s := testService()
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); _ = s.Start() }()
		go func() { defer wg.Done(); s.Close() }()
		go func() { defer wg.Done(); s.Close() }()
		wg.Wait()
		if err := s.Start(); err == nil {
			t.Fatal("closed service restarted")
		}
		if err := s.beginRequest(); err == nil {
			s.requests.release()
			t.Fatal("closed service admitted request")
		}
	}
}
