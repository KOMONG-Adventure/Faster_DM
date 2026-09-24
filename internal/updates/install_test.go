package updates

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type assetTransport func(*http.Request) (*http.Response, error)

func (f assetTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInstallerDownloadIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		mismatch, missing, duplicate bool
	}{
		{name: "verified"}, {name: "corrupt installer", mismatch: true}, {name: "missing checksum", missing: true}, {name: "duplicate checksum", duplicate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := t.TempDir()
			t.Setenv("LOCALAPPDATA", cache)
			t.Setenv("XDG_CACHE_HOME", cache)
			payload := strings.Repeat("installer data", 1024)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
			line := hash + "  FasterDM-Setup-1.2.3-x64.exe\n"
			if tc.missing {
				line = hash + "  other.exe\n"
			}
			if tc.duplicate {
				line += line
			}
			original := http.DefaultTransport
			http.DefaultTransport = assetTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Scheme != "https" || req.URL.Host != "github.com" {
					t.Fatal("unexpected asset host")
				}
				body := payload
				if strings.HasSuffix(req.URL.Path, "SHA256SUMS.txt") {
					body = line
				} else if tc.mismatch {
					body += "corruption"
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = original })
			path, err := DownloadInstaller(context.Background(), "v1.2.3", func(Progress) {})
			if tc.mismatch || tc.missing || tc.duplicate {
				if err == nil || path != "" {
					t.Fatal("unverified installer accepted")
				}
				files, _ := filepath.Glob(filepath.Join(cache, "FasterDM", "updates", "install-*", "*.exe"))
				if len(files) > 0 {
					t.Fatal("unverified executable not removed")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != payload {
					t.Fatal("installer bytes differ")
				}
			}
		})
	}
}
