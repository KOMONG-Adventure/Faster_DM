package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestChunk503ReducesConnectionsAndPreservesFile(t *testing.T) {
	data := payload(512 << 10)
	var rejected atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-0" && rejected.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(503)
			return
		}
		serveRange(w, r, data)
	}))
	defer server.Close()
	e := testEngine(t, func(c *Config) { c.Workers = 2 })
	updates := make(chan Snapshot, 256)
	path := filepath.Join(t.TempDir(), "recovered.zip")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := e.Download(ctx, server.URL, path, updates); err != nil {
		t.Fatal(err)
	}
	assertFile(t, path, data)
	close(updates)
	waitShown := false
	for p := range updates {
		if p.Message != "" && p.RetryInSeconds > 0 && p.ConnectionLimit == 1 {
			waitShown = true
		}
	}
	if !waitShown {
		t.Fatal("missing recovery countdown or connection reduction")
	}
}

func TestRetryAfterAndCancellation(t *testing.T) {
	for _, header := range []string{"120", time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)} {
		resp := &http.Response{StatusCode: 503, Header: http.Header{"Retry-After": []string{header}}}
		cause := statusError(resp)
		if delay := retryDelay(0, cause); delay < 119*time.Second {
			t.Fatalf("Retry-After ignored: %v", delay)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := backoff(ctx, 0, cause); err != context.Canceled {
			t.Fatal(err)
		}
	}
}
