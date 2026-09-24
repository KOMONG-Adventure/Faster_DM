// Package updates нь GitHub-ийн хамгийн сүүлийн stable release-ийг шалгана.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Release build-д -ldflags "-X .../internal/updates.Version=0.2.0" ашиглаж өөрчилнө.
var Version = "0.4.1"

const ReleasesURL = "https://github.com/KOMONG-Adventure/Faster_DM/releases"
const LatestAPI = "https://api.github.com/repos/KOMONG-Adventure/Faster_DM/releases/latest"

type Result struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	CheckedAt   string `json:"checkedAt"`
	PublishedAt string `json:"publishedAt"`
}

var stableVersion = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:\+[0-9A-Za-z.-]+)?$`)

func parseVersion(value string) ([3]uint64, error) {
	var parts [3]uint64
	match := stableVersion.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return parts, fmt.Errorf("Хувилбарын дугаар v1.2.3 хэлбэртэй байх ёстой: %s", value)
	}
	for i := range parts {
		n, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return parts, err
		}
		parts[i] = n
	}
	return parts, nil
}

func Check(ctx context.Context) Result {
	return check(ctx, &http.Client{Timeout: 12 * time.Second}, LatestAPI, Version)
}

func check(ctx context.Context, client *http.Client, endpoint, current string) Result {
	r := Result{Current: current, Status: "error", CheckedAt: time.Now().Format(time.RFC3339)}
	local, err := parseVersion(current)
	if err != nil {
		r.Message = err.Error()
		return r
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		r.Message = "Шалгах хүсэлт үүсгэж чадсангүй."
		return r
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "FasterDM/"+current)
	resp, err := client.Do(req)
	if err != nil {
		r.Message = "GitHub-тэй холбогдож чадсангүй. Интернэтээ шалгаад дахин оролдоно уу."
		return r
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		r.Status = "unavailable"
		r.Message = "Нийтэд нээлттэй release олдсонгүй. Release нийтлэгдээгүй эсвэл репозиторий хувийн тохиргоотой байж болно."
		return r
	case http.StatusForbidden, http.StatusTooManyRequests:
		r.Message = "GitHub шалгах эрх эсвэл хүсэлтийн хязгаарт хүрлээ. Түр хүлээгээд дахин шалгана уу."
		return r
	case http.StatusOK:
	default:
		r.Message = fmt.Sprintf("GitHub HTTP %d буцаалаа. Дараа дахин шалгана уу.", resp.StatusCode)
		return r
	}
	var release struct {
		Tag         string `json:"tag_name"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		r.Message = "GitHub-ийн хариуг уншиж чадсангүй."
		return r
	}
	if release.Draft || release.Prerelease {
		r.Status = "unavailable"
		r.Message = "Тогтвортой шинэ хувилбар хараахан нийтлэгдээгүй байна."
		return r
	}
	remote, err := parseVersion(release.Tag)
	if err != nil {
		r.Message = "Release-ийн хувилбарын дугаар танигдсангүй. GitHub Releases хуудсаас шалгана уу."
		return r
	}
	r.Latest = release.Tag
	r.PublishedAt = release.PublishedAt
	r.Status = "current"
	r.Message = "Та хамгийн сүүлийн хувилбарыг ашиглаж байна."
	for i := range local {
		if remote[i] > local[i] {
			r.Status = "available"
			r.Message = "Шинэ хувилбар бэлэн байна. Татаж суулгах товчийг дарна уу. Суулгах үед апп хаагдаж, дараа нь дахин нээгдэнэ."
			break
		}
		if remote[i] < local[i] {
			r.Message = "Таны хувилбар нийтлэгдсэн release-ээс шинэ байна."
			break
		}
	}
	return r
}
