// Package media нь зөвхөн YouTube холбоосыг тусдаа yt-dlp процессоор татна.
package media

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/KOMONG-Adventure/Faster_DM/internal/diskspace"
	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
	"github.com/KOMONG-Adventure/Faster_DM/internal/source"
)

const format = "bv*[height<=1080][ext=mp4]+ba[ext=m4a]/b[height<=1080][ext=mp4]/b[height<=1080]"

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func IsYouTube(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "youtu.be" || h == "youtube.com" || strings.HasSuffix(h, ".youtube.com")
}
func canonical(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	id := u.Query().Get("v")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Hostname() == "youtu.be" && len(parts) > 0 {
		id = parts[0]
	}
	if len(parts) > 1 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
		id = parts[1]
	}
	if !IsYouTube(raw) || !videoID.MatchString(id) {
		return "", errors.New("Нэг YouTube видеоны холбоос оруулна уу; playlist/channel холбоос дэмжихгүй.")
	}
	return "https://www.youtube.com/watch?v=" + id, nil
}

func toolsFolder() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(executable)
	candidates := []string{filepath.Join(dir, "tools"), filepath.Join(dir, "..", "build", "bin", "tools")}
	for _, candidate := range candidates {
		valid := true
		for _, name := range []string{"yt-dlp.exe", "deno.exe", "ffmpeg.exe", "ffprobe.exe"} {
			info, err := os.Stat(filepath.Join(candidate, name))
			if err != nil || info.IsDir() {
				valid = false
				break
			}
		}
		if valid {
			return filepath.Clean(candidate), nil
		}
	}
	return "", errors.New("YouTube хэрэгслүүд олдсонгүй. Аппын tools хавтсыг exe-тэй хамт хадгална уу; хөгжүүлэлтийн үед scripts/setup-media.ps1 ажиллуулна.")
}
func arguments(tools string) []string {
	return []string{"--ignore-config", "--no-plugin-dirs", "--no-playlist", "--no-warnings", "--socket-timeout", "20", "--retries", "3", "--js-runtimes", "deno:" + filepath.Join(tools, "deno.exe"), "--ffmpeg-location", tools, "--format", format, "--merge-output-format", "mp4", "--remux-video", "mp4"}
}

type boundedOutput struct {
	data  []byte
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if len(b.data) < b.limit {
		remaining := b.limit - len(b.data)
		b.data = append(b.data, p[:min(remaining, n)]...)
	}
	return n, nil
}
func readableError(raw []byte) error {
	message := strings.TrimSpace(string(raw))
	if len(message) > 1500 {
		message = message[len(message)-1500:]
	}
	if strings.Contains(message, "Sign in") || strings.Contains(message, "Private video") || strings.Contains(message, "confirm your age") {
		return errors.New("YouTube энэ видеонд нэвтрэх эрх эсвэл баталгаажуулалт шаардлаа. Нийтэд нээлттэй өөр холбоос ашиглана уу.")
	}
	if message == "" {
		message = "YouTube таталт амжилтгүй боллоо. Холбоос болон сүлжээгээ шалгана уу."
	}
	return errors.New(message)
}

type Info struct {
	Filename string
	Total    int64
}

func Probe(ctx context.Context, raw string) (Info, error) {
	link, err := canonical(raw)
	if err != nil {
		return Info{}, err
	}
	tools, err := toolsFolder()
	if err != nil {
		return Info{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	args := append(arguments(tools), "--skip-download", "--dump-single-json", "--", link)
	cmd := exec.CommandContext(ctx, filepath.Join(tools, "yt-dlp.exe"), args...)
	hideWindow(cmd)
	output := &boundedOutput{limit: 8 << 20}
	stderr := &boundedOutput{limit: 8192}
	cmd.Stdout = output
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Info{}, fmt.Errorf("YouTube мэдээлэл хүлээх хугацаа дууссан: %w", ctx.Err())
		}
		return Info{}, readableError(stderr.data)
	}
	type sizeInfo struct {
		Size   float64 `json:"filesize"`
		Approx float64 `json:"filesize_approx"`
	}
	var info struct {
		Title   string     `json:"title"`
		ID      string     `json:"id"`
		IsLive  bool       `json:"is_live"`
		Size    float64    `json:"filesize"`
		Approx  float64    `json:"filesize_approx"`
		Formats []sizeInfo `json:"requested_formats"`
	}
	if err := json.Unmarshal(output.data, &info); err != nil {
		return Info{}, errors.New("YouTube видеоны мэдээллийг уншиж чадсангүй.")
	}
	if info.IsLive {
		return Info{}, errors.New("Шууд дамжуулалт дэмжихгүй. Дууссан видеоны холбоос ашиглана уу.")
	}
	total := int64(max(info.Size, info.Approx))
	if len(info.Formats) > 0 {
		total = 0
		for _, f := range info.Formats {
			total += int64(max(f.Size, f.Approx))
		}
	}
	if total <= 0 {
		total = -1
	}
	if info.Title == "" {
		info.Title = info.ID
	}
	return Info{Filename: source.SafeName(info.Title + ".mp4"), Total: total}, nil
}

// Download нь pause үед процессоо хааж, resume үед тухайн .part-ийг --continue-гаар үргэлжлүүлнэ.
func Download(ctx context.Context, raw, destination string, expectedSize int64, control *download.Control, progress func(download.Snapshot)) (int64, error) {
	return DownloadWithOptions(ctx, raw, destination, expectedSize, control, Options{}, progress)
}

type Options struct {
	WorkDir   string
	RateLimit int64
}

func DownloadWithOptions(ctx context.Context, raw, destination string, expectedSize int64, control *download.Control, opts Options, progress func(download.Snapshot)) (result int64, returnErr error) {
	link, err := canonical(raw)
	if err != nil {
		return 0, err
	}
	tools, err := toolsFolder()
	if err != nil {
		return 0, err
	}
	work := opts.WorkDir
	if work == "" {
		work, err = os.MkdirTemp(filepath.Dir(destination), ".fasterdm-media-")
	} else {
		err = os.MkdirAll(work, 0700)
	}
	if err != nil {
		return 0, err
	}
	lock, err := os.OpenFile(filepath.Join(work, "session.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return 0, err
	}
	if err = download.LockPartial(lock); err != nil {
		lock.Close()
		return 0, err
	}
	defer func() {
		lock.Close()
		if opts.WorkDir == "" || returnErr == nil {
			os.RemoveAll(work)
		}
	}()
	parts := make(map[string]int64)
	var existing int64
	_ = filepath.WalkDir(work, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, e := d.Info(); e == nil {
				existing += info.Size()
			}
		}
		return nil
	})
	if expectedSize > (1<<63-1)/2 {
		return 0, diskspace.ErrLow
	}
	if err := diskspace.Check(work, max(0, max(expectedSize, 0)*2-existing)); err != nil {
		return 0, err
	}
	ctx, stopSpace := context.WithCancelCause(ctx)
	spaceDone := make(chan struct{})
	go func() {
		defer close(spaceDone)
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := diskspace.Check(work, 0); err != nil {
					stopSpace(err)
					return
				}
			}
		}
	}()
	defer func() {
		cause := context.Cause(ctx)
		stopSpace(nil)
		<-spaceDone
		if returnErr != nil && diskspace.IsFull(cause) {
			returnErr = cause
		}
	}()
	for {
		rctx, release, err := control.RequestContext(ctx)
		if err != nil {
			return 0, err
		}
		args := append(arguments(tools), "--continue", "--newline", "--no-colors", "--progress", "--progress-delta", "0.25", "--progress-template", `download:__FDM__{"format":%(info.format_id)j,"progress":%(progress)j}`, "--output", filepath.Join(work, "media.%(ext)s"), "--", link)
		if opts.RateLimit > 0 {
			args = append([]string{"--limit-rate", fmt.Sprint(opts.RateLimit)}, args...)
		}
		cmd := exec.CommandContext(rctx, filepath.Join(tools, "yt-dlp.exe"), args...)
		hideWindow(cmd)
		cmd.WaitDelay = 3 * time.Second
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			release()
			return 0, err
		}
		stderr := &boundedOutput{limit: 8192}
		cmd.Stderr = stderr
		if err := cmd.Start(); err != nil {
			release()
			return 0, err
		}
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 65536), 2<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "__FDM__") {
				continue
			}
			var event struct {
				Format   string `json:"format"`
				Progress struct {
					Downloaded float64 `json:"downloaded_bytes"`
					Speed      float64 `json:"speed"`
				} `json:"progress"`
			}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "__FDM__")), &event) != nil {
				continue
			}
			parts[event.Format] = max(parts[event.Format], int64(event.Progress.Downloaded))
			var downloaded int64
			for _, n := range parts {
				downloaded += n
			}
			total := expectedSize
			if total > 0 {
				total = max(total, downloaded)
			}
			progress(download.Snapshot{Total: total, Downloaded: downloaded, BytesPerSecond: event.Progress.Speed, Status: "downloading", Chunks: []download.ChunkSnapshot{}})
		}
		readErr := scanner.Err()
		if readErr != nil {
			_ = cmd.Process.Kill()
		}
		err = cmd.Wait()
		paused := download.PausedContext(rctx)
		release()
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if paused {
			continue
		}
		if err != nil {
			message := strings.ToLower(string(stderr.data))
			if strings.Contains(message, "no space left") || strings.Contains(message, "not enough space on the disk") || strings.Contains(message, "disk full") {
				return 0, diskspace.ErrLow
			}
			return 0, readableError(stderr.data)
		}
		if readErr != nil {
			return 0, readErr
		}
		break
	}
	path := filepath.Join(work, "media.mp4")
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return 0, fmt.Errorf("Бэлэн MP4 файл олдсонгүй: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return 0, err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return 0, err
	}
	if err = f.Close(); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := os.Link(path, destination); err != nil {
		return 0, err
	}
	return info.Size(), nil
}

var _ io.Writer = (*boundedOutput)(nil)
