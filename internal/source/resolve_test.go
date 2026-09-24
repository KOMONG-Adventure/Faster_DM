package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAutomaticNames(t *testing.T) {
	for _, tc := range []struct{ path, disposition, kind, want string }{{"/download", `attachment; filename="archive.zip"`, "application/octet-stream", "archive.zip"}, {"/download", "", "image/jpeg", "download.jpg"}, {"/movie.mp4?token=x", "", "video/mp4", "movie.mp4"}, {"/x", `attachment; filename*=UTF-8''%D0%B7%D1%83%D1%80%D0%B0%D0%B3.jpg`, "image/jpeg", "зураг.jpg"}, {"/", "", "audio/mpeg", "download.mp3"}} {
		t.Run(tc.want, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.kind)
				w.Header().Set("Content-Disposition", tc.disposition)
				w.Write([]byte("x"))
			}))
			defer s.Close()
			got, err := Filename(context.Background(), s.URL+tc.path)
			if err != nil || got != tc.want {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
}
func TestSafeName(t *testing.T) {
	for _, name := range []string{"../../file.zip", `C:\folder\photo.jpg`, "CON.txt", strings.Repeat("Монгол", 80) + ".mp4"} {
		got := SafeName(name)
		if strings.ContainsAny(got, `/\:`) || len(got) > 180 {
			t.Fatal(got)
		}
	}
}
