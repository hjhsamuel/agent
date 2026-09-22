package app

import (
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestServeHTTPReportsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{Addr: listener.Addr().String()}
	defer server.Close()
	done := make(chan error, 1)
	go func() { done <- serveHTTP(server, make(chan os.Signal)) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "serve HTTP") {
			t.Fatalf("expected startup error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("listener failure waited for an exit signal")
	}
}

func TestServeHTTPStopsOnSignal(t *testing.T) {
	server := &http.Server{Addr: "127.0.0.1:0"}
	defer server.Close()
	sig := make(chan os.Signal, 1)
	sig <- os.Interrupt
	done := make(chan error, 1)
	go func() { done <- serveHTTP(server, sig) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}
