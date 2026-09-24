//go:build windows

package main

import (
	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/KOMONG-Adventure/Faster_DM/internal/jobs"
	"log"
)

func initNotifications() {
	if err := toast.SetAppData(toast.AppData{AppID: "Faster DM", GUID: "{7442671B-FE16-48CB-8F03-89835AF7A8D4}"}); err != nil {
		log.Printf("notification setup: %v", err)
	}
}
func notifyDownload(j jobs.Job) {
	title := "Таталт дууслаа"
	if j.Status == "failed" {
		title = "Таталт амжилтгүй"
	}
	n := toast.Notification{AppID: "Faster DM", Title: title, Body: j.Filename}
	if err := n.Push(); err != nil {
		log.Printf("notification: %v", err)
	}
}
