package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
)

var historyID = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func NewPersistent(ctx context.Context, emit func(Job), dir string) (*Manager, error) {
	m := New(ctx, emit)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return m, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".session.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err == nil {
		err = download.LockPartial(lock)
	}
	if err != nil {
		if lock != nil {
			lock.Close()
		}
		m.closed = true
		m.cancel()
		return m, fmt.Errorf("Түүхийг түгжиж чадсангүй. Faster DM өөр цонхонд нээлттэй эсэхийг шалгана уу: %w", err)
	}
	m.historyLock = lock
	m.historyDir = dir
	files, err := os.ReadDir(dir)
	if err != nil {
		return m, err
	}
	var problems []error
	for _, file := range files {
		if file.Name() == "settings.json" || file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, file.Name())
		job, err := readHistory(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", file.Name(), err))
			continue
		}
		if !historyID.MatchString(job.ID) || file.Name() != job.ID+".json" || !filepath.IsAbs(job.Path) {
			problems = append(problems, fmt.Errorf("буруу түүхийн бичлэг: %s", file.Name()))
			continue
		}
		if isActive(job.Status) {
			job.Status = "failed"
			job.Error = "Апп хаагдсан тул таталт тасарсан. Холбоосоор дахин эхлүүлнэ үү."
			if job.URL != "" {
				job.Status = "paused"
				job.Error = ""
			}
		}
		job.Restored = true
		job.Revision++
		job.Progress.Status = job.Status
		job.Progress.BytesPerSecond = 0
		job.Progress.NetworkBytesPerSecond = 0
		job.Progress.DiskBytesPerSecond = 0
		job.Progress.ActiveConnections = 0
		job.Progress.Message = ""
		job.Progress.RetryInSeconds = 0
		job.Progress.Chunks = []download.ChunkSnapshot{}
		job.ETASeconds = -1
		if job.Status == "complete" {
			job.ETASeconds = 0
		}
		m.entries[job.ID] = &entry{job: job}
	}
	if err := m.loadSettings(); err != nil {
		problems = append(problems, err)
	}
	return m, errors.Join(problems...)
}

func readHistory(path string) (Job, error) {
	var job Job
	f, err := os.Open(path)
	if err != nil {
		return job, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
	if err != nil {
		return job, err
	}
	if len(data) > 2<<20 {
		return job, errors.New("түүхийн бичлэг хэт том")
	}
	err = json.Unmarshal(data, &job)
	if err == nil && job.Status != "complete" && job.Status != "failed" && job.Status != "canceled" && !isActive(job.Status) {
		err = errors.New("танигдаагүй төлөв")
	}
	return job, err
}

// Нэг таталт нэг файл: зэрэг нээлттэй аппуудын түүх бие биеэ дарахгүй.
// Progress frame бүрийг биш, эхлэл ба эцсийн төлөвийг хадгална.
func (m *Manager) persist(job Job) error {
	m.persistMu.Lock()
	defer m.persistMu.Unlock()
	if m.historyDir == "" {
		return nil
	}
	if !historyID.MatchString(job.ID) {
		return errors.New("буруу түүхийн ID")
	}
	job.Progress.Chunks = []download.ChunkSnapshot{}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(m.historyDir, ".history-*.tmp")
	if err != nil {
		return err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replaceHistory(f.Name(), filepath.Join(m.historyDir, job.ID+".json"))
}

// Хуучин хувилбарын анхдагч хавтас дахь бэлэн файлуудыг жагсаалтад сэргээнэ.
// Файлыг нээж унших, зөөх, устгахгүй; .part болон холбоос файлуудыг оруулахгүй.
func (m *Manager) ImportCompleted(folder string) error {
	if !filepath.IsAbs(folder) {
		return nil
	}
	known := make(map[string]bool)
	m.mu.Lock()
	for _, e := range m.entries {
		job := e.job
		known[strings.ToLower(filepath.Clean(job.Path))] = true
	}
	m.mu.Unlock()
	var problems []error
	for _, category := range []string{"Videos", "Photos", "Archives", "Audio", "Documents", "Other"} {
		dir := filepath.Join(folder, category)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			problems = append(problems, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".") || strings.EqualFold(filepath.Ext(entry.Name()), ".part") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			key := strings.ToLower(filepath.Clean(path))
			if known[key] {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			id := fmt.Sprintf("import-%x", sha256.Sum256([]byte(key)))
			job := Job{ID: id, Filename: entry.Name(), Path: path, Category: category, CreatedAt: info.ModTime().Format(time.RFC3339), Revision: 1, Status: "complete", Restored: true, SourceKind: "imported", Progress: download.Snapshot{Total: info.Size(), Downloaded: info.Size(), Status: "complete", Chunks: []download.ChunkSnapshot{}}}
			if err = m.persist(job); err != nil {
				problems = append(problems, err)
			}
			m.mu.Lock()
			m.entries[id] = entryForHistory(job)
			m.mu.Unlock()
			known[key] = true
		}
	}
	return errors.Join(problems...)
}

func entryForHistory(job Job) *entry { return &entry{job: job} }
