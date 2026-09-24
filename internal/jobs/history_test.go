package jobs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHistorySurvivesRestart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "saved download") }))
	defer server.Close()
	dir := t.TempDir()
	m, err := NewPersistent(context.Background(), nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	j, err := m.Start(Request{URL: server.URL, Filename: "archive.zip", Folder: t.TempDir(), Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	j = awaitJob(t, m, j.ID)
	m.Close()
	m2, err := NewPersistent(context.Background(), nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	saved, err := m2.Get(j.ID)
	if err != nil || saved.Status != "complete" || saved.Path != j.Path || saved.Progress.Downloaded != 14 || !saved.Restored {
		t.Fatalf("history: %+v, %v", saved, err)
	}
	if saved.Progress.BytesPerSecond != 0 {
		t.Fatal("stale speed restored")
	}
	data, err := os.ReadFile(saved.Path)
	if err != nil || string(data) != "saved download" {
		t.Fatal("download changed")
	}
}

func TestHistoryCorruptionAndInterruptedJob(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewPersistent(context.Background(), nil, dir)
	job := Job{ID: "test-interrupted", Path: filepath.Join(t.TempDir(), "file.zip"), Status: "downloading"}
	if err := m.persist(job); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("broken"), 0600)
	m.Close()
	m2, err := NewPersistent(context.Background(), nil, dir)
	if err == nil {
		t.Fatal("corrupt history not reported")
	}
	defer m2.Close()
	saved, err := m2.Get(job.ID)
	if err != nil || saved.Status != "failed" || isActive(saved.Status) {
		t.Fatal(saved, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bad.json")); err != nil {
		t.Fatal("corrupt file discarded")
	}
}

func TestHistoryImportsExistingFilesWithoutDuplicates(t *testing.T) {
	folder := t.TempDir()
	os.Mkdir(filepath.Join(folder, "Videos"), 0700)
	os.WriteFile(filepath.Join(folder, "Videos", "old.mp4"), []byte("video"), 0600)
	os.WriteFile(filepath.Join(folder, "Videos", "unfinished.part"), []byte("part"), 0600)
	dir := t.TempDir()
	m, _ := NewPersistent(context.Background(), nil, dir)
	if err := m.ImportCompleted(folder); err != nil {
		t.Fatal(err)
	}
	m.Close()
	m2, err := NewPersistent(context.Background(), nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m2.Close()
	if err := m2.ImportCompleted(folder); err != nil {
		t.Fatal(err)
	}
	list := m2.List()
	if len(list) != 1 || list[0].Filename != "old.mp4" || list[0].Progress.Total != 5 || list[0].SourceKind != "imported" {
		t.Fatal(list)
	}
}
