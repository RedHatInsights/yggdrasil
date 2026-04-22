package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInitHTTPClientTimeouts(t *testing.T) {
	initHTTPClient(nil, "test")

	transport := client.Transport.(*http.Transport)
	if transport.IdleConnTimeout != 30*time.Second {
		t.Errorf("IdleConnTimeout: got %v, want %v", transport.IdleConnTimeout, 30*time.Second)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf(
			"ResponseHeaderTimeout: got %v, want %v",
			transport.ResponseHeaderTimeout,
			30*time.Second,
		)
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v, want %v", client.Timeout, 30*time.Second)
	}
}

func TestHTTPClientResponseHeaderTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	initHTTPClient(nil, "test")
	// Override to a short timeout to keep the test fast.
	client.Transport.(*http.Transport).ResponseHeaderTimeout = time.Millisecond
	client.Timeout = 0 // remove to isolate ResponseHeaderTimeout

	if _, err := get(slow.URL); err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
