package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testEngine(t *testing.T, change func(*Config)) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Workers, cfg.BufferSize, cfg.MinChunkSize = 4, 4096, 8192
	if change != nil {
		change(&cfg)
	}
	e, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	return e
}

func payload(n int) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = byte((i*31 + i/251) % 256)
	}
	return p
}

func serveRange(w http.ResponseWriter, r *http.Request, data []byte) {
	w.Header().Set("ETag", `"version-1"`)
	var a, b int
	if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &a, &b); err != nil {
		w.Write(data)
		return
	}
	if a < 0 || b >= len(data) || a > b {
		w.WriteHeader(416)
		return
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", a, b, len(data)))
	w.Header().Set("Content-Length", fmt.Sprint(b-a+1))
	w.WriteHeader(206)
	w.Write(data[a : b+1])
}

func assertFile(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("файлын өгөгдөл зөрсөн: got=%d want=%d", len(actual), len(expected))
	}
}

func assertClean(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("дутуу файл үлдсэн: %v", entries)
	}
}

func TestParallelDownload(t *testing.T) {
	data := payload((1 << 20) + 173)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("identity encoding шаардлагатай")
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			requests.Add(1)
			if r.Header.Get("If-Match") != `"version-1"` || r.Header.Get("If-Range") != `"version-1"` {
				t.Error("validator алга")
			}
		}
		serveRange(w, r, data)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "file.bin")
	// Уншигчгүй progress channel engine-ийг блоклож болохгүй.
	result, err := testEngine(t, nil).Download(context.Background(), server.URL, path, make(chan Snapshot))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Parallel || result.Bytes != int64(len(data)) || requests.Load() < 4 {
		t.Fatalf("буруу үр дүн: %+v, хүсэлт=%d", result, requests.Load())
	}
	assertFile(t, path, data)
}

func TestSchedulerStealsOnlyUnreservedTail(t *testing.T) {
	s := newScheduler(128*1024, 2, 8192)
	a, b := s.acquire(), s.acquire()
	_, reserved := s.reserve(a, 4096)
	s.advance(b, int(b.end-b.next))
	s.mark(b, "complete", false)
	oldEnd := a.end
	c := s.acquire()
	if c == nil || c.start < int64(reserved) || a.end != c.start || c.end != oldEnd {
		t.Fatalf("давхардсан эсвэл алдагдсан интервал: a=%+v c=%+v", a, c)
	}
	s.advance(a, reserved)
	if a.next > a.end {
		t.Fatal("бичилтийн нөөц хуваах хилээс гарсан")
	}
}

func TestWorkStealingAgainstSlowServer(t *testing.T) {
	data := payload(512 * 1024)
	var stolen atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var a, b int
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &a, &b)
		if a > 0 && a < len(data)/2 {
			stolen.Add(1)
		}
		if a == 0 && b > 0 {
			w.Header().Set("ETag", `"version-1"`)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", a, b, len(data)))
			w.Header().Set("Content-Length", fmt.Sprint(b-a+1))
			w.WriteHeader(206)
			for pos := a; pos <= b; pos += 4096 {
				if _, err := w.Write(data[pos:min(pos+4096, b+1)]); err != nil {
					return
				}
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					return
				case <-time.After(3 * time.Millisecond):
				}
			}
			return
		}
		serveRange(w, r, data)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "file")
	_, err := testEngine(t, func(c *Config) { c.Workers = 2 }).Download(context.Background(), server.URL, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stolen.Load() == 0 {
		t.Fatal("удаан worker-ийн сүүлийг хувааж аваагүй")
	}
	assertFile(t, path, data)
}

func TestDroppedConnectionResumesCommittedOffset(t *testing.T) {
	data := payload(64 * 1024)
	var cut atomic.Bool
	var resumed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=4096-65535" {
			resumed.Store(true)
		}
		if r.Header.Get("Range") != "bytes=0-0" && cut.CompareAndSwap(false, true) {
			conn, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			fmt.Fprintf(rw, "HTTP/1.1 206 Partial Content\r\nETag: \"version-1\"\r\nContent-Range: bytes 0-65535/65536\r\nContent-Length: 65536\r\n\r\n")
			rw.Write(data[:4096])
			rw.Flush()
			return
		}
		serveRange(w, r, data)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "file")
	_, err := testEngine(t, func(c *Config) { c.Workers = 1 }).Download(context.Background(), server.URL, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Load() {
		t.Fatal("хадгалсан offset-оос үргэлжлээгүй")
	}
	assertFile(t, path, data)
}

func TestSequentialFallback(t *testing.T) {
	for _, mode := range []string{"no-range", "no-etag", "weak-etag", "unknown-size", "empty"} {
		t.Run(mode, func(t *testing.T) {
			data := payload(65539)
			if mode == "empty" {
				data = nil
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if (mode == "no-etag" || mode == "weak-etag") && r.Header.Get("Range") != "" {
					if mode == "weak-etag" {
						w.Header().Set("ETag", `W/"v1"`)
					}
					w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(data)))
					w.Header().Set("Content-Length", "1")
					w.WriteHeader(206)
					w.Write(data[:1])
					return
				}
				if mode == "unknown-size" {
					w.(http.Flusher).Flush()
				} else {
					w.Header().Set("Content-Length", fmt.Sprint(len(data)))
				}
				w.Write(data)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "file")
			result, err := testEngine(t, nil).Download(context.Background(), server.URL, path, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.Parallel {
				t.Fatal("single stream горим шаардлагатай")
			}
			assertFile(t, path, data)
		})
	}
}

func TestInvalidRangeOrChangedFileNeverPublished(t *testing.T) {
	for _, mode := range []string{"wrong-range", "changed-etag", "ignored-range", "precondition", "encoded", "wrong-length", "not-found"} {
		t.Run(mode, func(t *testing.T) {
			data := payload(65536)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") == "bytes=0-0" {
					serveRange(w, r, data)
					return
				}
				w.Header().Set("ETag", `"version-1"`)
				switch mode {
				case "ignored-range":
					w.WriteHeader(200)
				case "precondition":
					w.WriteHeader(412)
				case "not-found":
					w.WriteHeader(404)
				case "encoded":
					w.Header().Set("Content-Encoding", "gzip")
					w.WriteHeader(206)
				default:
					cr := "bytes 0-65535/65536"
					if mode == "wrong-range" {
						cr = "bytes 1-65535/65536"
					}
					if mode == "changed-etag" {
						w.Header().Set("ETag", `"version-2"`)
					}
					w.Header().Set("Content-Range", cr)
					if mode == "wrong-length" {
						w.Header().Set("Content-Length", "3")
					}
					w.WriteHeader(206)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			_, err := testEngine(t, func(c *Config) { c.Workers = 1 }).Download(context.Background(), server.URL, filepath.Join(dir, "file"), nil)
			if err == nil {
				t.Fatal("алдаа хүлээсэн")
			}
			assertClean(t, dir)
		})
	}
}

func TestCancellationAndIdleTimeout(t *testing.T) {
	for _, idle := range []bool{false, true} {
		t.Run(fmt.Sprint(idle), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") == "bytes=0-0" {
					serveRange(w, r, payload(65536))
					return
				}
				w.Header().Set("ETag", `"version-1"`)
				w.Header().Set("Content-Range", "bytes 0-65535/65536")
				w.WriteHeader(206)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			dir := t.TempDir()
			e := testEngine(t, func(c *Config) {
				c.Workers = 1
				c.MaxRetries = 0
				if idle {
					c.IdleTimeout = 40 * time.Millisecond
				}
			})
			_, err := e.Download(ctx, server.URL, filepath.Join(dir, "file"), nil)
			if err == nil {
				t.Fatal("цуцлалт/timeout хүлээсэн")
			}
			if !idle && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			if idle && ctx.Err() != nil {
				t.Fatal("idle timeout ажиллаагүй")
			}
			assertClean(t, dir)
		})
	}
}

func TestExistingDestinationPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := testEngine(t, nil).Download(context.Background(), "http://127.0.0.1:1", path, nil)
	if !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
	assertFile(t, path, []byte("existing"))
}

func TestConcurrentPublishDoesNotOverwrite(t *testing.T) {
	data := payload(65536)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serveRange(w, r, data) }))
	defer server.Close()
	e := testEngine(t, nil)
	path := filepath.Join(t.TempDir(), "file")
	var wg sync.WaitGroup
	var success atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.Download(context.Background(), server.URL, path, nil)
			if err == nil {
				success.Add(1)
			} else if !errors.Is(err, os.ErrExist) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatalf("амжилттай нийтэлсэн тоо: %d", success.Load())
	}
	assertFile(t, path, data)
}

func TestTransientStatusAndRetryLimit(t *testing.T) {
	for _, alwaysFail := range []bool{false, true} {
		t.Run(fmt.Sprint(alwaysFail), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 || alwaysFail {
					w.WriteHeader(503)
					return
				}
				serveRange(w, r, []byte("ok"))
			}))
			defer server.Close()
			dir := t.TempDir()
			path := filepath.Join(dir, "file")
			_, err := testEngine(t, func(c *Config) { c.MaxRetries = 1 }).Download(context.Background(), server.URL, path, nil)
			if alwaysFail {
				if err == nil || requests.Load() != 2 {
					t.Fatalf("retry хязгаар: %v, %d", err, requests.Load())
				}
				assertClean(t, dir)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertFile(t, path, []byte("ok"))
			}
		})
	}
}

func TestContentRange(t *testing.T) {
	for _, value := range []string{"", "bytes */100", "bytes 1-0/100", "bytes 0-100/100", "bytes -1-3/100", "bytes +0-3/100", "bytes 0-9223372036854775807/9223372036854775807", "bytes 0-3/*", "bytes 0-3/10junk"} {
		if _, _, _, err := contentRange(value); err == nil {
			t.Errorf("буруу range зөвшөөрсөн: %s", value)
		}
	}
	a, b, total, err := contentRange("bytes 0-3/10")
	if err != nil || a != 0 || b != 4 || total != 10 {
		t.Fatal(a, b, total, err)
	}
}

func TestExtraChunkedRangeBytesRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=0-0" {
			serveRange(w, r, []byte("abc"))
			return
		}
		w.Header().Set("ETag", `"version-1"`)
		w.Header().Set("Content-Range", "bytes 0-2/3")
		w.WriteHeader(206)
		w.(http.Flusher).Flush()
		w.Write([]byte("abcd"))
	}))
	defer server.Close()
	dir := t.TempDir()
	_, err := testEngine(t, nil).Download(context.Background(), server.URL, filepath.Join(dir, "file"), nil)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("илүү байтыг зөвшөөрсөн: %v", err)
	}
	assertClean(t, dir)
}

func TestSequentialRetryRestartsEntireFile(t *testing.T) {
	var fullRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "6")
		if r.Header.Get("Range") != "" {
			return
		}
		if fullRequests.Add(1) == 1 {
			io.Copy(w, strings.NewReader("old"))
			return
		}
		io.Copy(w, strings.NewReader("newest"))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "file")
	_, err := testEngine(t, nil).Download(context.Background(), server.URL, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, path, []byte("newest"))
}
