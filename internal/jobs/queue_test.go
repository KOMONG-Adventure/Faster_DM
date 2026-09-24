package jobs

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitIdle(t *testing.T, m *Manager, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		running := m.entries[id].running
		m.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("worker did not stop")
}

func TestQueueRestartResumeAndRemoveHistory(t *testing.T) {
	data := bytes.Repeat([]byte("resume-safe-data"), 65536)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-v1"`)
		http.ServeContent(w, r, "archive.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	dir := t.TempDir()
	folder := t.TempDir()
	m, err := NewPersistent(context.Background(), nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err = m.SetSettings(Settings{Concurrent: 1, RateLimit: 256 << 10}); err != nil {
		t.Fatal(err)
	}
	first, err := m.Start(Request{URL: srv.URL, Filename: "a.zip", Folder: folder, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Start(Request{URL: srv.URL, Filename: "b.zip", Folder: folder, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "queued" {
		t.Fatalf("expected queued, got %s", second.Status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, _ := m.Get(first.ID)
		if j.Progress.Downloaded > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no download progress")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err = m.Pause(first.ID); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, m, first.ID)
	deadline = time.Now().Add(5 * time.Second)
	for {
		j, _ := m.Get(second.ID)
		if j.Status != "queued" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue failed to advance")
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.Close()
	m2, err := NewPersistent(context.Background(), nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	restored, _ := m2.Get(first.ID)
	if restored.Status != "paused" || restored.Progress.Downloaded == 0 {
		t.Fatalf("lost resumable job: %+v", restored)
	}
	if m2.Settings().Concurrent != 1 || m2.Settings().RateLimit != 256<<10 {
		t.Fatal("lost settings")
	}
	if err = m2.SetSettings(Settings{Concurrent: 1}); err != nil {
		t.Fatal(err)
	}
	if err = m2.Resume(first.ID); err != nil {
		t.Fatal(err)
	}
	j := awaitJob(t, m2, first.ID)
	waitIdle(t, m2, first.ID)
	if j.Status != "complete" {
		t.Fatal(j.Error)
	}
	got, err := os.ReadFile(j.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("resumed file mismatch", err)
	}
	if err = m2.Remove(j.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(j.Path); err != nil {
		t.Fatal("history removal deleted file", err)
	}
	m2.Close()
	m3, err := NewPersistent(context.Background(), nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m3.Close()
	if err = m3.ImportCompleted(folder); err != nil {
		t.Fatal(err)
	}
	for _, j := range m3.List() {
		if j.Filename == "a.zip" {
			t.Fatal("removed history reappeared")
		}
	}
	if _, err = os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatal(err)
	}
}
