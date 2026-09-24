package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type metadata struct {
	size     int64
	etag     string
	parallel bool
}
type responseError struct {
	status     int
	retryAfter time.Duration
}

func (e *responseError) Error() string { return fmt.Sprintf("HTTP %d", e.status) }

// Timer нь хүсэлтийн нийт хугацаа бус, өгөгдөл ирэхгүй удах хугацааг хязгаарлана.
type idleBody struct {
	io.ReadCloser
	timer   *time.Timer
	cancel  context.CancelFunc
	timeout time.Duration
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.timer.Reset(b.timeout)
	}
	return n, err
}
func (b *idleBody) Close() error { b.timer.Stop(); b.cancel(); return b.ReadCloser.Close() }

func (e *Engine) get(ctx context.Context, url, byteRange, etag string) (*http.Response, error) {
	rctx, cancel := context.WithCancel(ctx)
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "FasterDM/0.1")
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	if etag != "" {
		req.Header.Set("If-Match", etag)
		req.Header.Set("If-Range", etag)
	}
	timer := time.AfterFunc(e.cfg.IdleTimeout, cancel)
	resp, err := e.client.Do(req)
	if err != nil {
		timedOut := rctx.Err() != nil
		timer.Stop()
		cancel()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Манай idle timer-ээр тасарсан хүсэлтийг дахин оролдож болно.
		if timedOut {
			return nil, fmt.Errorf("өгөгдөл хүлээх хугацаа дууссан: %w", context.DeadlineExceeded)
		}
		return nil, err
	}
	resp.Body = &idleBody{ReadCloser: resp.Body, timer: timer, cancel: cancel, timeout: e.cfg.IdleTimeout}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: Content-Encoding=%s", ErrProtocol, encoding)
	}
	return resp, nil
}

func contentRange(value string) (start, end, total int64, err error) {
	err = ErrProtocol
	if !strings.HasPrefix(value, "bytes ") {
		return
	}
	parts := strings.Split(strings.TrimPrefix(value, "bytes "), "/")
	if len(parts) != 2 {
		return
	}
	bounds := strings.Split(parts[0], "-")
	if len(bounds) != 2 {
		return
	}
	for _, number := range []string{bounds[0], bounds[1], parts[1]} {
		if number == "" {
			return
		}
		for _, digit := range number {
			if digit < '0' || digit > '9' {
				return
			}
		}
	}
	a, e1 := strconv.ParseInt(bounds[0], 10, 64)
	b, e2 := strconv.ParseInt(bounds[1], 10, 64)
	t, e3 := strconv.ParseInt(parts[1], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || a < 0 || b < a || t <= b {
		return
	}
	return a, b + 1, t, nil
}

func statusError(resp *http.Response) error {
	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrChanged
	}
	d := time.Duration(0)
	if raw := resp.Header.Get("Retry-After"); raw != "" {
		if seconds, err := strconv.ParseInt(raw, 10, 32); err == nil && seconds > 0 {
			d = time.Duration(seconds) * time.Second
		} else if when, err := http.ParseTime(raw); err == nil {
			d = time.Until(when)
		}
	}
	return &responseError{status: resp.StatusCode, retryAfter: d}
}

func (e *Engine) probe(ctx context.Context, url string) (metadata, error) {
	for attempt := 0; ; attempt++ {
		m, err := e.probeOnce(ctx, url)
		if err == nil {
			return m, nil
		}
		if attempt >= e.cfg.MaxRetries || !retryable(err) {
			return m, err
		}
		if err = backoff(ctx, attempt, err); err != nil {
			return m, err
		}
	}
}

func (e *Engine) probeOnce(ctx context.Context, url string) (metadata, error) {
	r, err := e.get(ctx, url, "bytes=0-0", "")
	if err != nil {
		return metadata{}, err
	}
	defer r.Body.Close()
	m := metadata{size: r.ContentLength}
	switch r.StatusCode {
	case http.StatusPartialContent:
		a, b, total, err := contentRange(r.Header.Get("Content-Range"))
		if err != nil || a != 0 || b != 1 || (r.ContentLength != -1 && r.ContentLength != 1) {
			return m, ErrProtocol
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 2))
		if err != nil {
			return m, err
		}
		if len(body) == 0 {
			return m, io.ErrUnexpectedEOF
		}
		if len(body) != 1 {
			return m, ErrProtocol
		}
		m.size = total
		m.etag = r.Header.Get("ETag")
		// Strong ETag байхгүй үед хүсэлт хооронд файл өөрчлөгдөөгүйг батлахгүй.
		m.parallel = len(m.etag) >= 2 && strings.HasPrefix(m.etag, "\"") && strings.HasSuffix(m.etag, "\"")
		return m, nil
	case http.StatusOK:
		return m, nil
	case http.StatusRequestedRangeNotSatisfiable:
		if r.Header.Get("Content-Range") == "bytes */0" {
			return metadata{size: 0}, nil
		}
	}
	return m, statusError(r)
}

func retryable(err error) bool {
	if errors.Is(err, ErrProtocol) || errors.Is(err, ErrChanged) {
		return false
	}
	var status *responseError
	if errors.As(err, &status) {
		return status.status == 408 || status.status == 429 || status.status == 500 || status.status == 502 || status.status == 503 || status.status == 504
	}
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func backoff(ctx context.Context, attempt int, cause error) error {
	delay := 250 * time.Millisecond * time.Duration(1<<min(attempt, 5))
	delay += time.Duration(rand.Int64N(int64(delay/2) + 1))
	var status *responseError
	if errors.As(cause, &status) && status.retryAfter > delay {
		delay = status.retryAfter
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
