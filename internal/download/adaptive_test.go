package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestConnectionTuner(t *testing.T) {
	tuner := connectionTuner{limit: 4, maximum: 32}
	for i, sample := range []struct {
		rate          float64
		retries, want int
	}{
		{100, 0, 8}, {160, 0, 8}, {160, 0, 16},
		{162, 0, 8}, // өсөлтгүй бол буцаана
		{160, 0, 8}, // дахин шууд холболт нэмэхгүй
		{80, 1, 4},  // retry өсвөл ачааллыг бууруулна
	} {
		if got := tuner.sample(sample.rate, sample.retries); got != sample.want {
			t.Fatalf("sample %d: connections=%d want=%d", i, got, sample.want)
		}
	}
}

func TestAdaptiveDownloadAndTelemetry(t *testing.T) {
	data := payload((64 << 20) + 173)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) }))
	defer server.Close()
	e := testEngine(t, func(c *Config) { c.Adaptive = true; c.Workers = 8; c.BufferSize = 512 << 10; c.MinChunkSize = 1 << 20 })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "large.zip")
	updates := make(chan Snapshot, 1024)
	result, err := e.Download(ctx, server.URL, path, updates)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Parallel {
		t.Fatal("parallel download expected")
	}
	assertFile(t, path, data)
	close(updates)
	measured := false
	for snapshot := range updates {
		if snapshot.DiskMeasured && snapshot.NetworkBytesPerSecond > 0 && snapshot.DiskBytesPerSecond > 0 {
			measured = true
		}
	}
	if !measured {
		t.Fatal("network and disk measurements missing")
	}
}

func TestReleasedChunkPreservesWrittenPrefix(t *testing.T) {
	s := newScheduler(1<<20, 1, 4096)
	c := s.acquire()
	s.reserve(c, 4096)
	s.advance(c, 2048)
	s.release(c)
	resumed := s.acquire()
	start, _ := s.bounds(resumed)
	if resumed != c || start != 2048 {
		t.Fatalf("resume offset=%d", start)
	}
}
