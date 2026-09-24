package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
)

type Settings struct {
	Concurrent    int   `json:"concurrent"`
	RateLimit     int64 `json:"rateLimit"`
	Notifications bool  `json:"notifications"`
}

func (m *Manager) Settings() Settings { m.mu.Lock(); defer m.mu.Unlock(); return m.settings }
func validSettings(s Settings) bool {
	return s.Concurrent >= 1 && s.Concurrent <= 4 && s.RateLimit >= 0 && s.RateLimit <= 1<<30
}
func (m *Manager) loadSettings() error {
	b, err := os.ReadFile(filepath.Join(m.historyDir, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var s Settings
	if json.Unmarshal(b, &s) != nil || !validSettings(s) {
		return errors.New("Тохиргооны файл гэмтсэн")
	}
	m.settings = s
	return nil
}
func (m *Manager) SetSettings(s Settings) error {
	if !validSettings(s) {
		return errors.New("Зэрэг таталт 1–4, хурд 0–1024 MiB/s байна")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.ctx.Err() != nil {
		return errors.New("Апп хаагдаж байна")
	}
	if m.historyDir != "" {
		b, _ := json.Marshal(s)
		f, err := os.CreateTemp(m.historyDir, ".settings-*")
		if err != nil {
			return err
		}
		defer func() { f.Close(); os.Remove(f.Name()) }()
		if _, err = f.Write(b); err != nil {
			return err
		}
		if err = f.Sync(); err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		if err = replaceHistory(f.Name(), filepath.Join(m.historyDir, "settings.json")); err != nil {
			return err
		}
	}
	m.settings = s
	m.dispatchLocked()
	return nil
}

func (m *Manager) partialPath(j Job) string {
	return filepath.Join(filepath.Dir(j.Path), ".fasterdm-"+j.ID+".part")
}
func (m *Manager) mediaPath(j Job) string {
	return filepath.Join(filepath.Dir(j.Path), ".fasterdm-media-"+j.ID)
}
func (m *Manager) removePartial(j Job) {
	if p := m.partialPath(j); p != "" {
		os.Remove(p)
		os.Remove(p + ".json")
	}
	if p := m.mediaPath(j); p != "" {
		os.RemoveAll(p)
	}
}

// Called with mu held. A slot remains occupied until every worker has exited.
func (m *Manager) dispatchLocked() {
	if m.closed || m.ctx.Err() != nil {
		return
	}
	active := 0
	var queue []*entry
	for _, e := range m.entries {
		if e.running {
			active++
		} else if e.job.Status == "queued" {
			queue = append(queue, e)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		if queue[i].job.CreatedAt == queue[j].job.CreatedAt {
			return queue[i].job.ID < queue[j].job.ID
		}
		return queue[i].job.CreatedAt < queue[j].job.CreatedAt
	})
	for _, e := range queue {
		if active >= m.settings.Concurrent {
			break
		}
		active++
		ctx, cancel := context.WithCancel(m.ctx)
		e.cancel = cancel
		e.control = download.NewControl()
		e.running = true
		e.started = time.Now()
		e.rateAt = e.started
		e.baseElapsed = e.job.ElapsedSeconds
		e.pausedDuration = 0
		e.pausedAt = time.Time{}
		e.job.Status = "probing"
		e.job.Restored = false
		e.job.Error = ""
		e.job.RateLimit = m.settings.RateLimit
		e.job.Revision++
		m.wg.Add(1)
		go m.run(ctx, e.job, e.job.URL, e.control)
	}
}

func (m *Manager) changeState(id, action string) error {
	m.mu.Lock()
	e, ok := m.entries[id]
	if !ok || m.closed {
		m.mu.Unlock()
		return errors.New("Таталт олдсонгүй эсвэл апп хаагдаж байна")
	}
	if !isActive(e.job.Status) && !(action == "resume" && e.job.Status == "failed" && e.job.URL != "") {
		m.mu.Unlock()
		return nil
	}
	switch action {
	case "pause":
		if e.job.Status == "canceling" {
			m.mu.Unlock()
			return nil
		}
		e.job.Status = "paused"
		if e.running {
			e.cancel()
		}
	case "resume":
		if e.running {
			m.mu.Unlock()
			return errors.New("Таталтыг хадгалж байна. Түр хүлээгээд үргэлжлүүлнэ үү.")
		}
		if e.job.Status != "paused" && e.job.Status != "failed" {
			m.mu.Unlock()
			return nil
		}
		e.job.Status = "queued"
	case "cancel":
		if e.running {
			e.job.Status = "canceling"
			e.cancel()
		} else {
			e.job.Status = "canceled"
			m.removePartial(e.job)
		}
	}
	e.job.Progress.BytesPerSecond = 0
	e.job.Progress.NetworkBytesPerSecond = 0
	e.job.Progress.DiskBytesPerSecond = 0
	e.job.Progress.ActiveConnections = 0
	e.job.Revision++
	job := e.job
	err := m.persist(job)
	m.dispatchLocked()
	m.mu.Unlock()
	if m.emit != nil && m.ctx.Err() == nil {
		m.emit(job)
	}
	return err
}
func (m *Manager) Pause(id string) error  { return m.changeState(id, "pause") }
func (m *Manager) Resume(id string) error { return m.changeState(id, "resume") }
func (m *Manager) Cancel(id string) error { return m.changeState(id, "cancel") }

// A tombstone prevents legacy-file import from recreating a deleted history row.
// The downloaded file itself is never deleted.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[id]
	if !ok {
		return errors.New("Таталт олдсонгүй")
	}
	if e.running || isActive(e.job.Status) {
		return errors.New("Эхлээд таталтыг цуцална уу")
	}
	j := e.job
	j.Hidden = true
	j.URL = ""
	j.Revision++
	if err := m.persist(j); err != nil {
		return err
	}
	m.removePartial(j)
	e.job = j
	return nil
}
func (m *Manager) Retry(id string) (Job, error) {
	j, err := m.Get(id)
	if err != nil {
		return Job{}, err
	}
	if isActive(j.Status) {
		return Job{}, errors.New("Таталт идэвхтэй байна")
	}
	if j.URL == "" {
		return Job{}, errors.New("Хуучин таталтын холбоос хадгалагдаагүй байна")
	}
	return m.Start(Request{URL: j.URL, Filename: j.Filename, Folder: filepath.Dir(filepath.Dir(j.Path)), Workers: j.Workers})
}
