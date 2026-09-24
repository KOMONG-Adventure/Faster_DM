package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Engine struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) (*Engine, error) {
	if cfg.Workers < 1 || cfg.Workers > 128 {
		return nil, errors.New("Workers нь 1–128 байх ёстой")
	}
	if cfg.BufferSize < 4096 || cfg.BufferSize > 4<<20 {
		return nil, errors.New("BufferSize нь 4 KiB–4 MiB байх ёстой")
	}
	if cfg.MinChunkSize < int64(cfg.BufferSize) || cfg.MinChunkSize > 1<<40 {
		return nil, errors.New("MinChunkSize нь BufferSize-аас багагүй, 1 TiB-аас ихгүй байна")
	}
	if cfg.MaxRetries < 0 || cfg.MaxRetries > 20 {
		return nil, errors.New("MaxRetries нь 0–20 байх ёстой")
	}
	if cfg.IdleTimeout <= 0 || cfg.ProgressInterval < time.Second/60 {
		return nil, errors.New("IdleTimeout эерэг, ProgressInterval нь 1/60 секундээс багагүй байна")
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DialContext:       (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2: true, DisableCompression: true,
			MaxIdleConns: cfg.Workers * 2, MaxIdleConnsPerHost: cfg.Workers,
			MaxConnsPerHost: cfg.Workers, IdleConnTimeout: 90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: cfg.IdleTimeout,
		}}
	}
	return &Engine{cfg: cfg, client: client}, nil
}

// Close нь engine-ийн сул HTTP холболтуудыг хаана. Өгсөн Client мөн хамрагдана.
func (e *Engine) Close() { e.client.CloseIdleConnections() }

// Download нь блоклодог тул Wails bridge үүнийг тусдаа goroutine-оос дуудна.
// updates нь сонголттой, илгээх ажиллагаа блоклохгүй; хэрэглэгч Download буцтал хааж болохгүй.
// Эцсийн үр дүнг Result/error-оос авна: дүүрсэн channel-ийн snapshot алгасагдаж болно.
// Байгаа destination-ийг хэзээ ч дарж бичихгүй. Алдаатай .part файлыг цэвэрлэнэ.
func (e *Engine) Download(ctx context.Context, rawURL, destination string, updates chan<- Snapshot) (result Result, returnErr error) {
	started := time.Now()
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return result, errors.New("http/https URL шаардлагатай; URL-д нэвтрэх мэдээлэл бүү оруул")
	}
	if destination == "" {
		return result, errors.New("хадгалах зам хоосон байна")
	}
	path, err := filepath.Abs(destination)
	if err != nil {
		return result, err
	}
	if _, err := os.Lstat(path); err == nil {
		return result, os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	meta, err := e.probe(ctx, rawURL)
	if err != nil {
		return result, fmt.Errorf("сервер шалгах: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".fasterdm-*.part")
	if err != nil {
		return result, fmt.Errorf("түр файл үүсгэх: %w", err)
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	// Windows/Linux дээр disk allocation-ийг таталт эхлэхээс өмнө нөөцөлнө.
	if meta.parallel || meta.size == 0 {
		if err := allocate(f, meta.size); err != nil {
			return result, fmt.Errorf("дискний зай нөөцлөх: %w", err)
		}
	}
	s := newScheduler(max(meta.size, 0), e.cfg.Workers, e.cfg.MinChunkSize)
	if !meta.parallel {
		s = newScheduler(max(meta.size, 0), 1, e.cfg.MinChunkSize)
		s.total, s.chunks[0].end = meta.size, meta.size
	}
	stop, stopped := make(chan struct{}), make(chan struct{})
	emit := func(status string) {
		if updates != nil {
			snapshot := s.snapshot(status)
			select {
			case updates <- snapshot:
			default:
			}
		}
	}
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(e.cfg.ProgressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				emit("downloading")
			}
		}
	}()
	defer func() {
		close(stop)
		<-stopped
		status := "complete"
		if returnErr != nil {
			status = "failed"
			if ctx.Err() != nil {
				status = "canceled"
			}
		}
		emit(status)
	}()
	emit("downloading")
	if meta.size == 0 {
		s.mark(s.chunks[0], "complete", false)
	} else if meta.parallel {
		err = e.parallel(ctx, rawURL, meta, f, s)
	} else {
		err = e.sequential(ctx, rawURL, f, s)
	}
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	final := s.snapshot("complete")
	if final.Downloaded != final.Total {
		return result, fmt.Errorf("%w: нийт байтын тоо тохирохгүй", ErrProtocol)
	}
	if err := f.Sync(); err != nil {
		return result, fmt.Errorf("диск рүү хадгалах: %w", err)
	}
	if err := f.Close(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// Ижил filesystem дээр hard link үүсгэх нь хуулбаргүй бөгөөд байгаа файлыг дарахгүй.
	// NTFS болон POSIX filesystem дэмжинэ; FAT/exFAT дээр аюулгүйгээр алдаа буцаана.
	if err := os.Link(f.Name(), path); err != nil {
		return result, fmt.Errorf("эцсийн файл нийтлэх (hard link дэмжлэг шаардлагатай): %w", err)
	}
	return Result{Path: path, Bytes: final.Downloaded, Parallel: meta.parallel, Duration: time.Since(started)}, nil
}

func (e *Engine) parallel(ctx context.Context, url string, meta metadata, f *os.File, s *scheduler) error {
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	for i := 0; i < e.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, e.cfg.BufferSize)
			for workCtx.Err() == nil {
				c := s.acquire()
				if c == nil {
					return
				}
				err := e.downloadChunk(workCtx, url, meta, f, s, c, buf)
				if err != nil {
					s.mark(c, "failed", false)
					once.Do(func() { firstErr = err; cancel() })
					return
				}
				s.mark(c, "complete", false)
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func (e *Engine) downloadChunk(ctx context.Context, url string, meta metadata, f *os.File, s *scheduler, c *chunk, buf []byte) error {
	for attempt := 0; ; {
		if err := e.cfg.Control.Wait(ctx); err != nil {
			return err
		}
		start, end := s.bounds(c)
		if start == end {
			return nil
		}
		s.mark(c, "downloading", false)
		err := e.rangeOnce(ctx, url, meta, f, s, c, buf, start, end)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errPaused) {
			s.mark(c, "paused", false)
			continue
		}
		if attempt >= e.cfg.MaxRetries || !retryable(err) {
			return fmt.Errorf("chunk %d: %w", c.id, err)
		}
		s.mark(c, "retrying", true)
		if err := backoff(ctx, attempt, err); err != nil {
			return err
		}
		attempt++
	}
}

func (e *Engine) rangeOnce(ctx context.Context, url string, meta metadata, f *os.File, s *scheduler, c *chunk, buf []byte, start, end int64) error {
	r, err := e.get(ctx, url, fmt.Sprintf("bytes=%d-%d", start, end-1), meta.etag)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusOK {
		return ErrChanged
	}
	if r.StatusCode != http.StatusPartialContent {
		return statusError(r)
	}
	a, b, size, err := contentRange(r.Header.Get("Content-Range"))
	if err != nil || a != start || b != end || size != meta.size || (r.ContentLength >= 0 && r.ContentLength != end-start) {
		return ErrProtocol
	}
	if r.Header.Get("ETag") != meta.etag {
		return ErrChanged
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		offset, capacity := s.reserve(c, len(buf))
		if capacity == 0 {
			// Хуваалцаагүй хүсэлтийг бүхэлд нь уншсан бол илүү байт ирээгүйг шалгана.
			// Хулгайлуулсан сүүлийг уншихгүй, Body.Close-оор холболтыг суллана.
			if offset == end {
				n, err := r.Body.Read(buf[:1])
				if n != 0 {
					return ErrProtocol
				}
				if !errors.Is(err, io.EOF) {
					if err != nil {
						return err
					}
					return ErrProtocol
				}
			}
			return nil
		}
		n, readErr := io.ReadFull(r.Body, buf[:capacity])
		if n > 0 {
			written, writeErr := f.WriteAt(buf[:n], offset)
			s.advance(c, written)
			if writeErr != nil {
				return fmt.Errorf("дискэнд бичих: %w", writeErr)
			}
			if written != n {
				return io.ErrShortWrite
			}
		} else {
			s.advance(c, 0)
		}
		if readErr != nil {
			return readErr
		}
	}
}

// Range/strong ETag байхгүй үед нэг GET-ийн бүтэн хариуг ашиглана.
// Холболт тасарвал өөр хувилбаруудыг нийлүүлэхгүйн тулд эхнээс нь дахин татна.
func (e *Engine) sequential(ctx context.Context, url string, f *os.File, s *scheduler) error {
	buf := make([]byte, e.cfg.BufferSize)
	c := s.chunks[0]
	for attempt := 0; ; {
		if err := e.cfg.Control.Wait(ctx); err != nil {
			return err
		}
		s.mark(c, "downloading", false)
		err := e.sequentialOnce(ctx, url, f, s, c, buf)
		if err == nil {
			s.mark(c, "complete", false)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errPaused) {
			s.mark(c, "paused", false)
			continue
		}
		if attempt >= e.cfg.MaxRetries || !retryable(err) {
			return err
		}
		s.mark(c, "retrying", true)
		if err := backoff(ctx, attempt, err); err != nil {
			return err
		}
		attempt++
	}
}

func (e *Engine) sequentialOnce(ctx context.Context, url string, f *os.File, s *scheduler, c *chunk, buf []byte) error {
	r, err := e.get(ctx, url, "", "")
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return statusError(r)
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if r.ContentLength >= 0 {
		if err := allocate(f, r.ContentLength); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.total, c.end, c.next, c.reserved = r.ContentLength, r.ContentLength, 0, 0
	s.previous[c.id] = 0
	s.mu.Unlock()
	var offset int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := r.Body.Read(buf)
		if n > 0 {
			if r.ContentLength >= 0 && int64(n) > r.ContentLength-offset {
				return ErrProtocol
			}
			written, writeErr := f.WriteAt(buf[:n], offset)
			offset += int64(written)
			s.advance(c, written)
			if writeErr != nil {
				return fmt.Errorf("дискэнд бичих: %w", writeErr)
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			if r.ContentLength >= 0 && offset != r.ContentLength {
				return io.ErrUnexpectedEOF
			}
			s.mu.Lock()
			s.total, c.end = offset, offset
			s.mu.Unlock()
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
