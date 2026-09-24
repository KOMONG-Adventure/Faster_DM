package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title: "Faster DM", Width: 1320, Height: 880, MinWidth: 1000, MinHeight: 720,
		BackgroundColour: &options.RGBA{R: 246, G: 248, B: 250, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup, OnShutdown: app.shutdown, OnBeforeClose: app.beforeClose,
		OnDomReady: app.startTray,
		Bind:       []interface{}{app},
	})
	if err != nil {
		// GUI build-д console байхгүй тул startup алдааг хэрэглэгчийн хавтсанд хадгална.
		if dir, e := os.UserConfigDir(); e == nil {
			os.MkdirAll(filepath.Join(dir, "FasterDM"), 0755)
			os.WriteFile(filepath.Join(dir, "FasterDM", "startup-error.log"), []byte(err.Error()), 0600)
		}
		log.Print(err)
	}
}
