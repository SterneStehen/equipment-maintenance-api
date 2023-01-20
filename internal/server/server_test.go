package server

import (
	"context"
	"net/http"
	"testing"
)

func TestNewConfiguresHTTPTimeouts(t *testing.T) {
	server := New(":8080", http.NewServeMux())

	if server.httpServer.ReadHeaderTimeout != readHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %s, want %s", server.httpServer.ReadHeaderTimeout, readHeaderTimeout)
	}
	if server.httpServer.ReadTimeout != readTimeout {
		t.Errorf("ReadTimeout = %s, want %s", server.httpServer.ReadTimeout, readTimeout)
	}
	if server.httpServer.WriteTimeout != writeTimeout {
		t.Errorf("WriteTimeout = %s, want %s", server.httpServer.WriteTimeout, writeTimeout)
	}
	if server.httpServer.IdleTimeout != idleTimeout {
		t.Errorf("IdleTimeout = %s, want %s", server.httpServer.IdleTimeout, idleTimeout)
	}
}

func TestRunStopsCleanlyWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	server := New("127.0.0.1:0", http.NewServeMux())
	if err := server.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}
