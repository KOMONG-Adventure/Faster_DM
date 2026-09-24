package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Progress struct {
	Phase      string `json:"phase"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}

var installerTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Installer-ийн URL-ийг frontend-ээс авахгүй: зөвхөн төслийн fixed release ашиглана.
func DownloadInstaller(ctx context.Context, tag string, progress func(Progress)) (path string, err error) {
	if !installerTag.MatchString(tag) {
		return "", errors.New("Installer-ийн хувилбар буруу байна.")
	}
	client := &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Hostname()
		if len(via) >= 10 || req.URL.Scheme != "https" || (host != "github.com" && !strings.HasSuffix(host, ".githubusercontent.com")) {
			return errors.New("Таталтын шилжүүлэх хаяг зөвшөөрөгдөөгүй.")
		}
		return nil
	}}
	name := "FasterDM-Setup-" + strings.TrimPrefix(tag, "v") + "-x64.exe"
	base := ReleasesURL + "/download/" + tag + "/"
	response, err := getAsset(ctx, client, base+"SHA256SUMS.txt")
	if err != nil {
		return "", err
	}
	manifest, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	response.Body.Close()
	if err != nil || len(manifest) > 64<<10 {
		return "", errors.New("Checksum manifest уншиж чадсангүй.")
	}
	expected, err := checksumFor(string(manifest), name)
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "FasterDM", "updates")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(dir, "install-")
	if err != nil {
		return "", err
	}
	path = filepath.Join(stage, name)
	cleanupPath := path
	defer func() {
		if err != nil {
			os.Remove(cleanupPath)
			os.Remove(stage)
		}
	}()
	response, err = getAsset(ctx, client, base+name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	const maxSize = 512 << 20
	if response.ContentLength > maxSize {
		return "", errors.New("Installer хэт том байна.")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	state := Progress{Phase: "downloading", Total: response.ContentLength}
	progress(state)
	last := time.Now()
	buf := make([]byte, 256<<10)
	for {
		n, readErr := response.Body.Read(buf)
		if n > 0 {
			state.Downloaded += int64(n)
			if state.Downloaded > maxSize {
				return "", errors.New("Installer хэт том байна.")
			}
			if _, err = file.Write(buf[:n]); err != nil {
				return "", err
			}
			hash.Write(buf[:n])
			if time.Since(last) >= 200*time.Millisecond {
				progress(state)
				last = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("Installer таталт тасарлаа: %w", readErr)
		}
	}
	state.Phase = "verifying"
	progress(state)
	if state.Downloaded == 0 || (state.Total >= 0 && state.Downloaded != state.Total) || hex.EncodeToString(hash.Sum(nil)) != expected {
		return "", errors.New("Checksum зөрсөн. Суулгагчийг ажиллуулсангүй. Дахин татаж оролдоно уу.")
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func getAsset(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FasterDM/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Шинэчлэлт татаж чадсангүй: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Installer татаж чадсангүй (HTTP %d). Release-ийн installer бүрэн нийтлэгдсэн эсэхийг шалгана уу.", resp.StatusCode)
	}
	return resp, nil
}

func checksumFor(manifest, name string) (string, error) {
	value := ""
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size || value != "" {
			return "", errors.New("Checksum manifest буруу байна.")
		}
		value = strings.ToLower(fields[0])
	}
	if value == "" {
		return "", errors.New("Installer-ийн checksum олдсонгүй.")
	}
	return value, nil
}
