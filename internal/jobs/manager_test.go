package jobs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCategoryAndFilenameSafety(t *testing.T) {
	for name, want := range map[string]string{"movie.MP4": "Videos", "photo.jpg": "Photos", "backup.tar.gz": "Archives", "track.flac": "Audio", "report.pdf": "Documents", "app.exe": "Other", "download": "Other"} {
		if got := Category(name); got != want {
			t.Errorf("%s: %s != %s", name, got, want)
		}
	}
	for _, name := range []string{"../file.zip", `..\file.zip`, "CON", "con.txt", "LPT1.txt", "COM¹.txt", "file.", "file ", "a:b.zip", "a\x00.txt"} {
		if err := validateFilename(name); err == nil {
			t.Errorf("аюултай нэр зөвшөөрсөн: %q", name)
		}
	}
	if err := validateFilename("Монгол зураг.png"); err != nil {
		t.Fatal(err)
	}
}

func awaitJob(t *testing.T, m *Manager, id string) Job {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("таталт дуусаагүй")
			return Job{}
		case <-ticker.C:
			j, err := m.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			if !isActive(j.Status) {
				return j
			}
		}
	}
}

func TestCategorizedDownloadAndDuplicateNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "test contents") }))
	defer server.Close()
	m := New(context.Background(), nil)
	defer m.Close()
	base := t.TempDir()
	for i := 0; i < 2; i++ {
		j, err := m.Start(Request{URL: server.URL, Filename: "archive.zip", Folder: base, Workers: 4})
		if err != nil {
			t.Fatal(err)
		}
		j = awaitJob(t, m, j.ID)
		if j.Status != "complete" {
			t.Fatal(j.Error)
		}
		name := "archive.zip"
		if i > 0 {
			name = "archive (1).zip"
		}
		if j.Path != filepath.Join(base, "Archives", name) {
			t.Fatal(j.Path)
		}
		b, err := os.ReadFile(j.Path)
		if err != nil || string(b) != "test contents" {
			t.Fatal(string(b), err)
		}
		if j.Progress.Downloaded != 13 || j.Progress.Total != 13 {
			t.Fatal(j.Progress)
		}
	}
}

func TestAutomaticallyNamedDownload(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="picture.jpg"`)
		fmt.Fprint(w, "photo")
	}))
	defer s.Close()
	m := New(context.Background(), nil)
	defer m.Close()
	j, err := m.Start(Request{URL: s.URL, Folder: t.TempDir(), Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	j = awaitJob(t, m, j.ID)
	if j.Status != "complete" || j.Filename != "picture.jpg" || j.Category != "Photos" {
		t.Fatal(j)
	}
}

func TestCancelAndShutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Length", "100000")
			return
		}
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	m := New(context.Background(), nil)
	base := t.TempDir()
	j, err := m.Start(Request{URL: server.URL, Filename: "movie.mp4", Folder: base, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(j.ID); err != nil {
		t.Fatal(err)
	}
	j = awaitJob(t, m, j.ID)
	if j.Status != "canceled" {
		t.Fatal(j.Status)
	}
	if _, err := os.Stat(j.Path); !os.IsNotExist(err) {
		t.Fatal("цуцалсан файлыг нийтэлсэн")
	}
	m.Close()
	if _, err := m.Start(Request{URL: server.URL, Filename: "new.zip", Folder: base, Workers: 4}); err == nil {
		t.Fatal("хаагдсан manager ажил зөвшөөрсөн")
	}
}

func TestInvalidRequestsAndFailedHTTP(t *testing.T) {
	m := New(context.Background(), nil)
	defer m.Close()
	for _, req := range []Request{{URL: "file:///test", Filename: "x", Folder: t.TempDir(), Workers: 4}, {URL: "https://example.com", Filename: "../x", Folder: t.TempDir(), Workers: 4}, {URL: "https://example.com", Filename: "x", Folder: t.TempDir(), Workers: 5}} {
		if _, err := m.Start(req); err == nil {
			t.Fatal("буруу хүсэлтийг зөвшөөрсөн")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "missing", 404) }))
	defer server.Close()
	j, err := m.Start(Request{URL: server.URL, Filename: "missing.pdf", Folder: t.TempDir(), Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	j = awaitJob(t, m, j.ID)
	if j.Status != "failed" || !strings.Contains(j.Error, "404") {
		t.Fatal(j)
	}
}
