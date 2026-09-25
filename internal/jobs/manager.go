// Package jobs нь dashboard-ийн таталт, ангилсан хавтас, lifecycle-ийг удирдана.
package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
	"github.com/KOMONG-Adventure/Faster_DM/internal/media"
	"github.com/KOMONG-Adventure/Faster_DM/internal/source"
)

type Request struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Folder   string `json:"folder"`
	Workers  int    `json:"workers"`
}

type Job struct {
	URL            string            `json:"url,omitempty"`
	Hidden         bool              `json:"hidden,omitempty"`
	Missing        bool              `json:"missing"`
	RateLimit      int64             `json:"rateLimit"`
	Restored       bool              `json:"restored"`
	ID             string            `json:"id"`
	Filename       string            `json:"filename"`
	Path           string            `json:"path"`
	Category       string            `json:"category"`
	CreatedAt      string            `json:"createdAt"`
	Revision       int64             `json:"revision"`
	Workers        int               `json:"workers"`
	Status         string            `json:"status"`
	Error          string            `json:"error"`
	Progress       download.Snapshot `json:"progress"`
	ElapsedSeconds float64           `json:"elapsedSeconds"`
	ETASeconds     float64           `json:"etaSeconds"`
	SourceKind     string            `json:"sourceKind"`
}

type entry struct {
	job            Job
	running        bool
	baseElapsed    float64
	cancel         context.CancelFunc
	control        *download.Control
	started        time.Time
	pausedAt       time.Time
	pausedDuration time.Duration
	speed          float64
	networkSpeed   float64
	diskSpeed      float64
	rateAt         time.Time
}
type Manager struct {
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	entries     map[string]*entry
	sequence    uint64
	emit        func(Job)
	closed      bool
	historyDir  string
	settings    Settings
	persistMu   sync.Mutex
	historyLock *os.File
}

func New(ctx context.Context, emit func(Job)) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	return &Manager{ctx: ctx, cancel: cancel, entries: make(map[string]*entry), emit: emit, settings: Settings{Concurrent: 2, Notifications: true}}
}

func Category(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	groups := map[string]string{
		"Videos":    ".mp4 .mkv .mov .avi .webm .m4v .wmv .flv .mpeg .mpg .ts",
		"Photos":    ".jpg .jpeg .png .gif .webp .svg .avif .heic .bmp .tif .tiff .ico .raw",
		"Archives":  ".zip .rar .7z .gz .bz2 .xz .tar .tgz .zst .iso",
		"Audio":     ".mp3 .wav .flac .aac .ogg .m4a .opus .wma .aiff",
		"Documents": ".pdf .doc .docx .xls .xlsx .ppt .pptx .txt .csv .md .epub .rtf .odt .json",
	}
	for category, extensions := range groups {
		for _, candidate := range strings.Fields(extensions) {
			if ext == candidate {
				return category
			}
		}
	}
	return "Other"
}

func DefaultFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads", "Faster DM")
}

func validateFilename(name string) error {
	if name == "" || name == "." || name == ".." || len(name) > 180 || strings.ContainsAny(name, `<>:"/\|?*`) || strings.TrimRight(name, ". ") != name {
		return errors.New("Файлын нэр буруу байна. Зам биш, зөвхөн нэр ба өргөтгөл оруулна уу.")
	}
	for _, r := range name {
		if r < 32 {
			return errors.New("Файлын нэрд тусгай удирдлагын тэмдэгт байж болохгүй.")
		}
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	base = strings.TrimRight(base, " ")
	port := strings.TrimPrefix(strings.TrimPrefix(base, "COM"), "LPT")
	reservedPort := port != base && strings.Contains("|1|2|3|4|5|6|7|8|9|¹|²|³|", "|"+port+"|")
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || reservedPort {
		return errors.New("Энэ файлын нэрийг Windows систем нөөцөлсөн. Өөр нэр сонгоно уу.")
	}
	return nil
}

func (m *Manager) Start(req Request) (Job, error) {
	req.URL = strings.TrimSpace(req.URL)
	req.Filename = strings.TrimSpace(req.Filename)
	u, err := url.Parse(req.URL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return Job{}, errors.New("Зөв HTTP эсвэл HTTPS холбоос оруулна уу.")
	}
	sourceKind := "http"
	total := int64(-1)
	if media.IsYouTube(req.URL) {
		sourceKind = "youtube"
		info, err := media.Probe(m.ctx, req.URL)
		if err != nil {
			return Job{}, err
		}
		total = info.Total
		if req.Filename == "" {
			req.Filename = info.Filename
		} else if !strings.EqualFold(filepath.Ext(req.Filename), ".mp4") {
			req.Filename = strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename)) + ".mp4"
		}
	} else if req.Filename == "" {
		name, err := source.Filename(m.ctx, req.URL)
		if err != nil {
			return Job{}, err
		}
		req.Filename = name
	}
	if err := validateFilename(req.Filename); err != nil {
		return Job{}, err
	}
	if req.Workers != 0 && req.Workers != 1 && req.Workers != 2 && req.Workers != 4 && req.Workers != 8 && req.Workers != 16 && req.Workers != 32 {
		return Job{}, errors.New("Автомат эсвэл 1, 2, 4, 8, 16, 32 холболт сонгоно уу.")
	}
	if req.Folder == "" {
		req.Folder = DefaultFolder()
	}
	if !filepath.IsAbs(req.Folder) {
		return Job{}, errors.New("Хадгалах үндсэн хавтсаа сонгоно уу.")
	}
	category := Category(req.Filename)
	dir := filepath.Join(filepath.Clean(req.Folder), category)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Job{}, errors.New("Апп хаагдаж байна.")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		m.mu.Unlock()
		return Job{}, fmt.Errorf("Хавтас үүсгэх боломжгүй: %w", err)
	}
	// Байгаа файл болон энэ session-ийн идэвхтэй destination-ийг давхардахгүй болгоно.
	name := req.Filename
	for suffix := 0; ; suffix++ {
		if suffix > 9999 {
			m.mu.Unlock()
			return Job{}, errors.New("Файлын өөр нэр сонгоно уу.")
		}
		if suffix > 0 {
			ext := filepath.Ext(req.Filename)
			name = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(req.Filename, ext), suffix, ext)
		}
		path := filepath.Join(dir, name)
		_, statErr := os.Lstat(path)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			m.mu.Unlock()
			return Job{}, statErr
		}
		reserved := false
		for _, e := range m.entries {
			if strings.EqualFold(e.job.Path, path) && (isActive(e.job.Status) || (!e.job.Hidden && e.job.Status == "failed" && e.job.URL != "")) {
				reserved = true
				break
			}
		}
		if errors.Is(statErr, os.ErrNotExist) && !reserved {
			break
		}
	}
	m.sequence++
	id := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), rand.Text())
	job := Job{ID: id, Filename: name, Path: filepath.Join(dir, name), Category: category, CreatedAt: time.Now().Format(time.RFC3339Nano), Revision: 1, Workers: req.Workers, Status: "queued", Progress: download.Snapshot{Total: -1, Status: "probing", Chunks: []download.ChunkSnapshot{}}}
	job.SourceKind = sourceKind
	job.URL = req.URL
	job.Progress.Total = total
	job.ETASeconds = -1
	m.entries[id] = &entry{job: job}
	if err := m.persist(job); err != nil {
		delete(m.entries, id)
		m.mu.Unlock()
		return Job{}, err
	}
	m.dispatchLocked()
	job = m.entries[id].job
	m.mu.Unlock()
	return job, nil
}

func isActive(status string) bool {
	return status == "queued" || status == "probing" || status == "downloading" || status == "canceling" || status == "paused"
}

func (m *Manager) update(id string, change func(*Job)) {
	m.mu.Lock()
	e := m.entries[id]
	// Өмнө нийтэлсэн snapshot-ийн slice-ийг дахин өөрчлөхгүй.
	e.job.Progress.Chunks = append([]download.ChunkSnapshot{}, e.job.Progress.Chunks...)
	change(&e.job)
	now := time.Now()
	elapsed := now.Sub(e.started) - e.pausedDuration
	if !e.pausedAt.IsZero() {
		elapsed -= now.Sub(e.pausedAt)
	}
	e.job.ElapsedSeconds = e.baseElapsed + math.Max(0, elapsed.Seconds())
	e.job.ETASeconds = -1
	if e.job.Status == "downloading" {
		alpha := 1 - math.Exp(-now.Sub(e.rateAt).Seconds()/2)
		e.speed += alpha * (e.job.Progress.BytesPerSecond - e.speed)
		e.networkSpeed += alpha * (e.job.Progress.NetworkBytesPerSecond - e.networkSpeed)
		e.diskSpeed += alpha * (e.job.Progress.DiskBytesPerSecond - e.diskSpeed)
		e.job.Progress.BytesPerSecond = e.speed
		e.job.Progress.NetworkBytesPerSecond = e.networkSpeed
		e.job.Progress.DiskBytesPerSecond = e.diskSpeed
		if e.speed > 1024 && e.job.Progress.Total > 0 {
			e.job.ETASeconds = math.Max(0, float64(e.job.Progress.Total-e.job.Progress.Downloaded)/e.speed)
		}
	} else {
		e.speed = 0
		e.job.Progress.Message = ""
		e.job.Progress.RetryInSeconds = 0
		e.networkSpeed, e.diskSpeed = 0, 0
		e.job.Progress.BytesPerSecond = 0
		e.job.Progress.NetworkBytesPerSecond, e.job.Progress.DiskBytesPerSecond = 0, 0
		e.job.Progress.ActiveConnections = 0
		if e.job.Status == "complete" {
			e.job.ETASeconds = 0
		}
		if e.job.Status == "paused" {
			for i := range e.job.Progress.Chunks {
				e.job.Progress.Chunks[i].BytesPerSecond = 0
				if e.job.Progress.Chunks[i].Status != "complete" {
					e.job.Progress.Chunks[i].Status = "paused"
				}
			}
		}
	}
	e.rateAt = now
	e.job.Revision++
	job := e.job
	if !isActive(job.Status) || job.Status == "paused" {
		if err := m.persist(job); err != nil {
			e.job.Error = "Түүх хадгалж чадсангүй: " + err.Error()
			job = e.job
		}
	}
	m.mu.Unlock()
	if m.emit != nil && m.ctx.Err() == nil {
		m.emit(job)
	}
}

func (m *Manager) run(ctx context.Context, job Job, source string, control *download.Control) {
	defer m.wg.Done()
	defer func() {
		m.mu.Lock()
		e := m.entries[job.ID]
		e.cancel()
		e.running = false
		if e.job.Status == "canceled" {
			m.removePartial(e.job)
		}
		m.dispatchLocked()
		m.mu.Unlock()
	}()
	m.update(job.ID, func(j *Job) {})
	if job.SourceKind == "youtube" {
		bytes, err := media.DownloadWithOptions(ctx, source, job.Path, job.Progress.Total, control, media.Options{WorkDir: m.mediaPath(job), RateLimit: job.RateLimit}, func(snapshot download.Snapshot) {
			m.update(job.ID, func(j *Job) {
				j.Progress = snapshot
				if j.Status != "paused" && j.Status != "canceling" && j.Status != "queued" {
					j.Status = "downloading"
				}
			})
		})
		m.update(job.ID, func(j *Job) {
			previous := j.Status
			if err != nil {
				j.Status = "failed"
				j.Error = err.Error()
				if ctx.Err() != nil {
					j.Status = "canceled"
					j.Error = "Таталтыг цуцалсан."
					if (m.ctx.Err() != nil && previous != "canceling") || previous == "paused" {
						j.Status = "paused"
						j.Error = ""
					}
				}
			} else {
				j.Status = "complete"
				j.Progress.Total = bytes
				j.Progress.Downloaded = bytes
			}
			j.Progress.Status = j.Status
		})
		return
	}
	cfg := download.DefaultConfig()
	cfg.Workers = job.Workers
	if cfg.Workers == 0 {
		cfg.Workers, cfg.Adaptive = 32, true
		cfg.BufferSize = 512 << 10
		cfg.MinChunkSize = 4 << 20
	}
	cfg.Control = control
	cfg.ResumePath = m.partialPath(job)
	if job.RateLimit > 0 {
		cfg.Limiter = &download.Limiter{BytesPerSecond: job.RateLimit}
		cfg.BufferSize = 32 << 10
	}
	engine, err := download.New(cfg)
	if err != nil {
		m.update(job.ID, func(j *Job) { j.Status = "failed"; j.Error = err.Error() })
		return
	}
	defer engine.Close()
	updates := make(chan download.Snapshot, 1)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for snapshot := range updates {
			m.update(job.ID, func(j *Job) {
				j.Progress = snapshot
				if j.Status != "canceling" && j.Status != "paused" && j.Status != "queued" {
					j.Status = "downloading"
				}
			})
		}
	}()
	result, err := engine.Download(ctx, source, job.Path, updates)
	close(updates)
	<-drained
	m.update(job.ID, func(j *Job) {
		j.Progress.BytesPerSecond = 0
		previous := j.Status
		if err != nil {
			j.Status = "failed"
			j.Error = err.Error()
			if ctx.Err() != nil {
				j.Status = "canceled"
				j.Error = "Таталтыг цуцалсан."
				if (m.ctx.Err() != nil && previous != "canceling") || previous == "paused" {
					j.Status = "paused"
					j.Error = ""
				}
			}
		} else {
			j.Status = "complete"
			j.Progress.Total = result.Bytes
			j.Progress.Downloaded = result.Bytes
		}
		j.Progress.Status = j.Status
		for i := range j.Progress.Chunks {
			j.Progress.Chunks[i].BytesPerSecond = 0
			if j.Progress.Chunks[i].Status != "complete" {
				j.Progress.Chunks[i].Status = j.Status
			}
		}
	})
}

func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Job, 0, len(m.entries))
	for _, e := range m.entries {
		job := e.job
		if job.Hidden {
			continue
		}
		if job.Status == "complete" {
			_, err := os.Stat(job.Path)
			job.Missing = errors.Is(err, os.ErrNotExist)
		}
		job.Progress.Chunks = append([]download.ChunkSnapshot{}, job.Progress.Chunks...)
		result = append(result, job)
	}
	sort.Slice(result, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, result[i].CreatedAt)
		b, _ := time.Parse(time.RFC3339, result[j].CreatedAt)
		if a.Equal(b) {
			return result[i].ID > result[j].ID
		}
		return a.After(b)
	})
	return result
}

func (m *Manager) Get(id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[id]; ok {
		job := e.job
		job.Progress.Chunks = append([]download.ChunkSnapshot{}, job.Progress.Chunks...)
		return job, nil
	}
	return Job{}, errors.New("Таталт олдсонгүй.")
}

func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		m.wg.Wait()
		return
	}
	m.closed = true
	m.cancel()
	for _, e := range m.entries {
		if !e.running && isActive(e.job.Status) {
			e.job.Status = "paused"
			_ = m.persist(e.job)
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
	if m.historyLock != nil {
		m.historyLock.Close()
	}
}
