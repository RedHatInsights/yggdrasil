package http

import (
	stdhttp "net/http"
	"testing"
	"time"
)

func TestSetIdleConnTimeout(t *testing.T) {
	client := NewHTTPClient(nil, "test")
	want := 42 * time.Second
	client.SetIdleConnTimeout(want)
	got := client.Transport.(*stdhttp.Transport).IdleConnTimeout
	if got != want {
		t.Errorf("SetIdleConnTimeout: got %v, want %v", got, want)
	}
}

func TestSetResponseHeaderTimeout(t *testing.T) {
	client := NewHTTPClient(nil, "test")
	want := 15 * time.Second
	client.SetResponseHeaderTimeout(want)
	got := client.Transport.(*stdhttp.Transport).ResponseHeaderTimeout
	if got != want {
		t.Errorf("SetResponseHeaderTimeout: got %v, want %v", got, want)
	}
}
