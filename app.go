package main

import (
	"context"
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/KOMONG-Adventure/Faster_DM/internal/jobs"
	"github.com/KOMONG-Adventure/Faster_DM/internal/updates"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	manager       *jobs.Manager
	trayReady     atomic.Bool
	closing       atomic.Bool
	trayOnce      sync.Once
	trayLifecycle sync.Mutex
	trayCancel    context.CancelFunc
	trayWG        sync.WaitGroup
	windowMu      sync.Mutex
	windowHidden  bool
	restoreUntil  time.Time
}
type State struct {
	Jobs   []jobs.Job `json:"jobs"`
	Folder string     `json:"folder"`
}

func NewApp() *App                             { return &App{} }
func (a *App) GetAppVersion() string           { return updates.Version }
func (a *App) CheckForUpdates() updates.Result { return updates.Check(a.ctx) }
func (a *App) OpenReleases()                   { wailsruntime.BrowserOpenURL(a.ctx, updates.ReleasesURL) }

// Clipboard-ийн ердийн текстийг frontend рүү дамжуулахгүй, зөвхөн HTTP(S) URL авна.
// Холбоосыг шалгах гэж сүлжээний хүсэлт илгээхгүй.
func (a *App) ClipboardDownloadURL() (string, error) {
	text, err := wailsruntime.ClipboardGetText(a.ctx)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if len(text) == 0 || len(text) > 16384 || strings.IndexFunc(text, unicode.IsSpace) >= 0 {
		return "", nil
	}
	u, err := url.Parse(text)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", nil
	}
	return text, nil
}
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager = jobs.New(ctx, func(job jobs.Job) { wailsruntime.EventsEmit(ctx, "download:changed", job) })
}
func (a *App) shutdown(context.Context) { a.closing.Store(true); a.stopTray(); a.manager.Close() }
func (a *App) MinimiseToTray() error {
	if !a.trayReady.Load() {
		return errors.New("Tray хараахан бэлэн болоогүй байна. Цонхны − товчийг ашиглана уу.")
	}
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	wailsruntime.WindowHide(a.ctx)
	a.windowHidden = true
	return nil
}
func (a *App) showFromTray() {
	if a.ctx == nil || a.closing.Load() {
		return
	}
	a.windowMu.Lock()
	defer a.windowMu.Unlock()
	a.restoreUntil = time.Now().Add(time.Second)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
	a.windowHidden = false
}
func (a *App) beforeClose(ctx context.Context) bool {
	active := false
	for _, j := range a.manager.List() {
		if j.Status == "probing" || j.Status == "downloading" || j.Status == "canceling" || j.Status == "paused" {
			active = true
			break
		}
	}
	if !active {
		return false
	}
	answer, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{Type: wailsruntime.QuestionDialog, Title: "Таталт үргэлжилж байна", Message: "Аппыг хаавал идэвхтэй таталтууд цуцлагдана. Хаах уу?", Buttons: []string{"Yes", "No"}, DefaultButton: "No", CancelButton: "No"})
	return err != nil || answer != "Yes"
}
func (a *App) GetState() State                                  { return State{Jobs: a.manager.List(), Folder: jobs.DefaultFolder()} }
func (a *App) StartDownload(req jobs.Request) (jobs.Job, error) { return a.manager.Start(req) }
func (a *App) CancelDownload(id string) error                   { return a.manager.Cancel(id) }
func (a *App) PauseDownload(id string) error                    { return a.manager.Pause(id) }
func (a *App) ResumeDownload(id string) error                   { return a.manager.Resume(id) }
func (a *App) ChooseFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Таталтуудын үндсэн хавтас сонгох"})
}
func (a *App) OpenFolder(id string) error {
	job, err := a.manager.Get(id)
	if err != nil {
		return err
	}
	dir := filepath.Dir(job.Path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
