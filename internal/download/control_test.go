package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestPauseResumeKeepsCommittedBytesWithoutRetryBudget(t *testing.T) {
	data := payload(512 * 1024)
	var resumed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var a, b int
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &a, &b)
		if a > 0 {
			resumed.Store(true)
		}
		if b == 0 {
			serveRange(w, r, data)
			return
		}
		w.Header().Set("ETag", `"version-1"`)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", a, b, len(data)))
		w.Header().Set("Content-Length", fmt.Sprint(b-a+1))
		w.WriteHeader(206)
		for offset := a; offset <= b; offset += 4096 {
			if _, err := w.Write(data[offset:min(offset+4096, b+1)]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(4 * time.Millisecond):
			}
		}
	}))
	defer server.Close()
	control := NewControl()
	e := testEngine(t, func(c *Config) { c.Control = control; c.Workers = 1; c.MaxRetries = 0 })
	updates := make(chan Snapshot, 64)
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "file")
	go func() { _, err := e.Download(ctx, server.URL, path, updates); done <- err; close(updates) }()
	for snapshot := range updates {
		if snapshot.Downloaded >= 8192 {
			break
		}
	}
	control.Pause()
	// Request цуцлагдаж buffer-ийн сүүлийн бичилт дуусахыг snapshot-оор батална.
	var stable int64
	for i := 0; i < 5; i++ {
		select {
		case snapshot := <-updates:
			stable = snapshot.Downloaded
		case err := <-done:
			t.Fatalf("pause үед download буцсан: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for i := 0; i < 3; i++ {
		select {
		case snapshot := <-updates:
			if snapshot.Downloaded != stable {
				t.Fatal("pause үед байт нэмэгдсэн")
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	control.Resume()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !resumed.Load() {
		t.Fatal("эхнээс бус offset-оос үргэлжлээгүй")
	}
	assertFile(t, path, data)
}

func TestCancelWhilePaused(t *testing.T) {
	c := NewControl()
	c.Pause()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Wait(ctx); err != context.Canceled {
		t.Fatal(err)
	}
}
