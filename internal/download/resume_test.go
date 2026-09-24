package download

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentResumeAndChangedSource(t *testing.T) {
	data := payload(2 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) }))
	defer srv.Close()
	dir := t.TempDir()
	partial := filepath.Join(dir, "download.part")
	dest := filepath.Join(dir, "file.zip")
	e := testEngine(t, func(c *Config) { c.ResumePath = partial; c.Limiter = &Limiter{BytesPerSecond: 1 << 20} })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan Snapshot, 1)
	done := make(chan error, 1)
	go func() { _, err := e.Download(ctx, srv.URL, dest, updates); done <- err }()
	timeout := time.After(5 * time.Second)
wait:
	for {
		select {
		case p := <-updates:
			if p.Downloaded >= 64<<10 {
				cancel()
				break wait
			}
		case <-timeout:
			t.Fatal("no progress")
		}
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	f, err := os.OpenFile(partial, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	s := newScheduler(int64(len(data)), 4, 8192)
	meta := metadata{size: int64(len(data)), etag: `"version-1"`, parallel: true}
	ok, err := restoreCheckpoint(partial+".json", srv.URL, meta, f, s)
	if err != nil || !ok || s.snapshot("paused").Downloaded == 0 {
		t.Fatalf("checkpoint: %v %v", ok, err)
	}
	meta.etag = `"version-2"`
	if _, err = restoreCheckpoint(partial+".json", srv.URL, meta, f, s); !errors.Is(err, ErrChanged) {
		t.Fatal("changed source accepted", err)
	}
	f.Close()
	e2 := testEngine(t, func(c *Config) { c.ResumePath = partial })
	if _, err = e2.Download(context.Background(), srv.URL, dest, nil); err != nil {
		t.Fatal(err)
	}
	assertFile(t, dest, data)
	if _, err = os.Stat(partial + ".json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("checkpoint not cleaned")
	}
}

func TestLimiterPacesAndCancels(t *testing.T) {
	l := &Limiter{BytesPerSecond: 10000}
	start := time.Now()
	if err := l.Wait(context.Background(), 1000); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 90*time.Millisecond {
		t.Fatal("rate limit bypassed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx, 1000000); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
