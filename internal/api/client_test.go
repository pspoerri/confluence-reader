package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient builds a Client wired to srv with sleep stubbed out so retries
// don't add real delay to tests.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient(srv.URL, "user", "token")
	c.HTTPClient = srv.Client()
	c.sleep = func(time.Duration) {}
	return c
}

func TestClient_Retries429(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results": []}`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if _, err := c.do("GET", "/test", nil); err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
}

func TestClient_Retries5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	if _, err := c.do("GET", "/test", nil); err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected 2 calls, got %d", got)
	}
}

func TestClient_AuthErrorNoRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.do("GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if ae.Status != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", ae.Status)
	}
	if !strings.Contains(ae.Error(), "configure") {
		t.Errorf("expected error to mention 'configure', got %q", ae.Error())
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 call (no retry on auth error), got %d", got)
	}
}

func TestClient_NonRetryable4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.do("GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if ae.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", ae.Status)
	}
}

func TestClient_HTMLBodyTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, "<!DOCTYPE html><html>...</html>")
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 0 // skip retry, see the raw error
	_, err := c.do("GET", "/test", nil)
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if strings.Contains(ae.Body, "<") {
		t.Errorf("expected HTML to be replaced with status text, got %q", ae.Body)
	}
}

func TestClient_MaxRetriesExceeded(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 2 // 3 attempts total
	if _, err := c.do("GET", "/test", nil); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	fallback := 7 * time.Second
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"5", 5 * time.Second},
		{"  10  ", 10 * time.Second},
		{"0", 0},
		{"", fallback},
		{"nope", fallback},
	}
	for _, tt := range tests {
		if got := parseRetryAfter(tt.in, fallback); got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBackoff_CappedAtMax(t *testing.T) {
	c := &Client{MaxBackoff: 4 * time.Second}
	if got := c.backoff(0); got != time.Second {
		t.Errorf("attempt 0: got %v, want 1s", got)
	}
	if got := c.backoff(10); got != 4*time.Second {
		t.Errorf("attempt 10: got %v, want cap 4s", got)
	}
}
