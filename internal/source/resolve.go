// Package source нь серверийн header болон холбоосоос файлын нэрийг тодорхойлно.
package source

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/KOMONG-Adventure/Faster_DM/internal/download"
)

func Filename(ctx context.Context, rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for attempt := 0; ; attempt++ {
		name, err := filenameOnce(ctx, rawURL)
		if err == nil || attempt >= 3 || !download.CanRetry(err) || ctx.Err() != nil {
			return name, err
		}
		if waitErr := download.WaitRetry(ctx, attempt, err); waitErr != nil {
			return "", fmt.Errorf("%w (хүлээх хугацаа дууссан эсвэл цуцалсан)", err)
		}
	}
}

func filenameOnce(ctx context.Context, rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "FasterDM/0.5")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Файлын нэр тодорхойлох: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if !(resp.StatusCode == 416 && resp.Header.Get("Content-Range") == "bytes */0") {
			return "", download.HTTPStatusError(resp)
		}
	}
	name := ""
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		name = params["filename"]
	}
	if name == "" {
		name = path.Base(resp.Request.URL.Path)
		if decoded, err := url.PathUnescape(name); err == nil {
			name = decoded
		}
	}
	if name == "" || name == "." || name == "/" {
		name = "download"
	}
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if contentType == "text/html" {
		return "", fmt.Errorf("Энэ холбоос файл биш, веб хуудас байна. Шууд файлын эсвэл YouTube видеоны холбоос оруулна уу.")
	}
	if path.Ext(name) == "" {
		extensions := map[string]string{"video/mp4": ".mp4", "video/webm": ".webm", "audio/mpeg": ".mp3", "audio/mp4": ".m4a", "image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif", "application/zip": ".zip", "application/x-7z-compressed": ".7z", "application/pdf": ".pdf", "text/plain": ".txt", "application/json": ".json", "application/gzip": ".gz"}
		ext := extensions[contentType]
		if ext == "" {
			ext = ".bin"
		}
		name += ext
	}
	return SafeName(name), nil
}

func SafeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" || name == "." {
		name = "download.bin"
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if len(ext) > 20 {
		ext = ".bin"
	}
	for len(base)+len(ext) > 170 {
		_, n := utf8.DecodeLastRuneInString(base)
		base = base[:len(base)-n]
	}
	upper := strings.ToUpper(strings.TrimSpace(base))
	if upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" || strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT") {
		base = "_" + base
	}
	return base + ext
}
