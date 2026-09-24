//go:build windows

package main

import (
	"context"
	_ "embed"
	"runtime"
	"time"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

func (a *App) startTray(context.Context) {
	a.trayLifecycle.Lock()
	defer a.trayLifecycle.Unlock()
	if a.closing.Load() {
		return
	}
	a.trayOnce.Do(func() {
		ctx, cancel := context.WithCancel(a.ctx)
		a.trayCancel = cancel
		go func() {
			// Tray-ийн HWND болон message loop ижил Windows thread дээр байна.
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			systray.Run(func() {
				systray.SetIcon(trayIcon)
				systray.SetTooltip("Faster DM · Таталт ард үргэлжилж байна")
				systray.SetOnClick(func(systray.IMenu) { go a.showFromTray() })
				systray.SetOnDClick(func(systray.IMenu) { go a.showFromTray() })
				systray.AddMenuItem("Faster DM нээх", "Dashboard нээх").Click(func() { go a.showFromTray() })
				systray.AddSeparator()
				systray.AddMenuItem("Аппаас гарах", "Идэвхтэй таталт байвал баталгаажуулна").Click(func() {
					go func() { a.showFromTray(); wailsruntime.Quit(a.ctx) }()
				})
				a.trayReady.Store(true)
				if a.closing.Load() {
					systray.Quit()
				}
			}, func() { a.trayReady.Store(false) })
		}()
		a.trayWG.Add(1)
		go func() {
			defer a.trayWG.Done()
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					if !a.trayReady.Load() || a.closing.Load() {
						continue
					}
					a.windowMu.Lock()
					if !a.windowHidden && now.After(a.restoreUntil) && wailsruntime.WindowIsMinimised(a.ctx) {
						wailsruntime.WindowHide(a.ctx)
						a.windowHidden = true
					}
					a.windowMu.Unlock()
				}
			}
		}()
	})
}

func (a *App) stopTray() {
	a.trayLifecycle.Lock()
	defer a.trayLifecycle.Unlock()
	if a.trayCancel != nil {
		a.trayCancel()
	}
	a.trayWG.Wait()
	if a.trayReady.Load() {
		systray.Quit()
	}
}
