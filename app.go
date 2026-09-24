package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/KOMONG-Adventure/Faster_DM/internal/jobs"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	manager *jobs.Manager
}
type State struct {
	Jobs   []jobs.Job `json:"jobs"`
	Folder string     `json:"folder"`
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager = jobs.New(ctx, func(job jobs.Job) { wailsruntime.EventsEmit(ctx, "download:changed", job) })
}
func (a *App) shutdown(context.Context) { a.manager.Close() }
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
