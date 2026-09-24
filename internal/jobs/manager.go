// Package jobs нь dashboard-ийн таталт, ангилсан хавтас, lifecycle-ийг удирдана.
package jobs

import (
	"context"
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
	cancel         context.CancelFunc
	control        *download.Control
	started        time.Time
	pausedAt       time.Time
	pausedDuration time.Duration
	speed          float64
	rateAt         time.Time
}
type Manager struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	entries  map[string]*entry
	sequence uint64
	emit     func(Job)
	closed   bool
}

func New(ctx context.Context, emit func(Job)) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	return &Manager{ctx: ctx, cancel: cancel, entries: make(map[string]*entry), emit: emit}
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
	if req.Workers != 4 && req.Workers != 8 && req.Workers != 16 && req.Workers != 32 {
		return Job{}, errors.New("Холболтын тоо 4, 8, 16 эсвэл 32 байна.")
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
	active := 0
	for _, e := range m.entries {
		if isActive(e.job.Status) {
			active++
		}
	}
	if active >= 4 {
		m.mu.Unlock()
		return Job{}, errors.New("Зэрэг 4 файл татаж болно. Нэг таталт дууссаны дараа нэмнэ үү.")
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
			if strings.EqualFold(e.job.Path, path) && isActive(e.job.Status) {
				reserved = true
				break
			}
		}
		if errors.Is(statErr, os.ErrNotExist) && !reserved {
			break
		}
	}
	m.sequence++
	id := fmt.Sprintf("%d-%d", time.Now().UnixMilli(), m.sequence)
	ctx, cancel := context.WithCancel(m.ctx)
	job := Job{ID: id, Filename: name, Path: filepath.Join(dir, name), Category: category, CreatedAt: time.Now().Format(time.RFC3339), Revision: 1, Workers: req.Workers, Status: "probing", Progress: download.Snapshot{Total: -1, Status: "probing", Chunks: []download.ChunkSnapshot{}}}
	job.SourceKind = sourceKind
	job.Progress.Total = total
	control := download.NewControl()
	job.ETASeconds = -1
	m.entries[id] = &entry{job: job, cancel: cancel, control: control, started: time.Now(), rateAt: time.Now()}
	m.wg.Add(1)
	m.mu.Unlock()
	go m.run(ctx, job, req.URL, control)
	return job, nil
}

func isActive(status string) bool {
	return status == "probing" || status == "downloading" || status == "canceling" || status == "paused"
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
	e.job.ElapsedSeconds = math.Max(0, elapsed.Seconds())
	e.job.ETASeconds = -1
	if e.job.Status == "downloading" {
		alpha := 1 - math.Exp(-now.Sub(e.rateAt).Seconds()/2)
		e.speed += alpha * (e.job.Progress.BytesPerSecond - e.speed)
		e.job.Progress.BytesPerSecond = e.speed
		if e.speed > 1024 && e.job.Progress.Total > 0 {
			e.job.ETASeconds = math.Max(0, float64(e.job.Progress.Total-e.job.Progress.Downloaded)/e.speed)
		}
	} else {
		e.speed = 0
		e.job.Progress.BytesPerSecond = 0
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
	m.mu.Unlock()
	if m.emit != nil && m.ctx.Err() == nil {
		m.emit(job)
	}
}

func (m *Manager) run(ctx context.Context, job Job, source string, control *download.Control) {
	defer m.wg.Done()
	defer func() { m.mu.Lock(); m.entries[job.ID].cancel(); m.mu.Unlock() }()
	if job.SourceKind == "youtube" {
		bytes, err := media.Download(ctx, source, job.Path, job.Progress.Total, control, func(snapshot download.Snapshot) {
			m.update(job.ID, func(j *Job) {
				j.Progress = snapshot
				if j.Status != "paused" && j.Status != "canceling" {
					j.Status = "downloading"
				}
			})
		})
		m.update(job.ID, func(j *Job) {
			if err != nil {
				j.Status = "failed"
				j.Error = err.Error()
				if ctx.Err() != nil {
					j.Status = "canceled"
					j.Error = "Таталтыг цуцалсан."
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
	cfg.Control = control
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
				if j.Status != "canceling" && j.Status != "paused" {
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
		if err != nil {
			j.Status = "failed"
			j.Error = err.Error()
			if ctx.Err() != nil {
				j.Status = "canceled"
				j.Error = "Таталтыг цуцалсан."
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
		job.Progress.Chunks = append([]download.ChunkSnapshot{}, job.Progress.Chunks...)
		result = append(result, job)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
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

func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	e, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("Таталт олдсонгүй.")
	}
	if !isActive(e.job.Status) {
		m.mu.Unlock()
		return nil
	}
	e.job.Status = "canceling"
	e.job.Revision++
	job := e.job
	e.cancel()
	m.mu.Unlock()
	if m.emit != nil && m.ctx.Err() == nil {
		m.emit(job)
	}
	return nil
}

func (m *Manager) Pause(id string) error  { return m.setPaused(id, true) }
func (m *Manager) Resume(id string) error { return m.setPaused(id, false) }
func (m *Manager) setPaused(id string, paused bool) error {
	m.mu.Lock()
	e, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("Таталт олдсонгүй.")
	}
	if !isActive(e.job.Status) || e.job.Status == "canceling" {
		m.mu.Unlock()
		return nil
	}
	now := time.Now()
	if paused && e.job.Status != "paused" {
		e.pausedAt = now
		e.job.Status = "paused"
		e.control.Pause()
	} else if !paused && e.job.Status == "paused" {
		e.pausedDuration += now.Sub(e.pausedAt)
		e.pausedAt = time.Time{}
		e.job.Status = "downloading"
		e.control.Resume()
	}
	e.speed = 0
	e.rateAt = now
	e.job.Progress.BytesPerSecond = 0
	e.job.ETASeconds = -1
	e.job.Revision++
	job := e.job
	m.mu.Unlock()
	if m.emit != nil && m.ctx.Err() == nil {
		m.emit(job)
	}
	return nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	m.mu.Unlock()
	m.wg.Wait()
}
