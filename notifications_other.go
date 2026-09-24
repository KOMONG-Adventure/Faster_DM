//go:build !windows

package main

import "github.com/KOMONG-Adventure/Faster_DM/internal/jobs"

func initNotifications()        {}
func notifyDownload(j jobs.Job) {}
