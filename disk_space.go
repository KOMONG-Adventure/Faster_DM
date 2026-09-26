package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/KOMONG-Adventure/Faster_DM/internal/browserbridge"
	"github.com/KOMONG-Adventure/Faster_DM/internal/diskspace"
	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
	"github.com/KOMONG-Adventure/Faster_DM/internal/jobs"
	"github.com/KOMONG-Adventure/Faster_DM/internal/media"
	"github.com/KOMONG-Adventure/Faster_DM/internal/source"
)

type DiskReport struct {
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	Total     int64  `json:"total"`
	Required  int64  `json:"required"`
	Free      int64  `json:"free"`
	Enough    bool   `json:"enough"`
	Estimated bool   `json:"estimated"`
}

func (a *App) CheckDiskSpace(req jobs.Request) (DiskReport, error) {
	r := DiskReport{Total: -1}
	req.URL = strings.TrimSpace(req.URL)
	if err := browserbridge.Validate(req.URL); err != nil {
		return r, err
	}
	if req.Folder == "" {
		req.Folder = jobs.DefaultFolder()
	}
	if !filepath.IsAbs(req.Folder) {
		return r, errors.New("Хадгалах хавтсаа сонгоно уу")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	r.Filename = strings.TrimSpace(req.Filename)
	if media.IsYouTube(req.URL) {
		info, err := media.Probe(ctx, req.URL)
		if err != nil {
			return r, err
		}
		r.Total = info.Total
		r.Estimated = true
		if r.Filename == "" {
			r.Filename = info.Filename
		}
		r.Filename = strings.TrimSuffix(r.Filename, filepath.Ext(r.Filename)) + ".mp4"
	} else {
		if r.Filename == "" {
			name, err := source.Filename(ctx, req.URL)
			if err != nil {
				return r, err
			}
			r.Filename = name
		}
		e, err := download.New(download.DefaultConfig())
		if err != nil {
			return r, err
		}
		defer e.Close()
		r.Total, err = e.InspectSize(ctx, req.URL)
		if err != nil {
			return r, err
		}
	}
	if filepath.Base(r.Filename) != r.Filename || strings.ContainsAny(r.Filename, `<>:"/\|?*`) {
		return r, errors.New("Файлын нэр буруу байна")
	}
	r.Path = filepath.Join(req.Folder, jobs.Category(r.Filename))
	free, err := diskspace.Free(r.Path)
	if err != nil {
		return r, err
	}
	r.Free = free
	needed := max(r.Total, 0)
	if r.Estimated {
		if needed > (1<<63-1)/2 {
			return r, errors.New("Файлын хэмжээ хэт том")
		}
		needed *= 2
	}
	if needed > 1<<63-1-diskspace.Reserve {
		return r, errors.New("Файлын хэмжээ хэт том")
	}
	r.Enough = diskspace.Enough(free, needed)
	r.Required = needed + diskspace.Reserve
	return r, nil
}
