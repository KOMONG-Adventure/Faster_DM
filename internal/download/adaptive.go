package download

import (
	"context"
	"os"
	"sync"
	"time"
)

// 3 секундийн цонхоор бичиж дуусгасан байт хэмжинэ. Шинэ холболт 8%-ийн
// өсөлт өгөхгүй бол буцааж, 30 секунд хүлээнэ. Энэ нь хурдны баталгаа биш.
type connectionTuner struct {
	limit, maximum, trialFrom, cooldown int
	baseline                            float64
}

func (t *connectionTuner) sample(rate float64, retries int) int {
	if retries > 0 {
		t.limit = max(min(4, t.maximum), t.limit/2)
		t.trialFrom, t.cooldown = 0, 10
		return t.limit
	}
	if t.trialFrom > 0 {
		if rate < t.baseline*1.08 {
			t.limit = t.trialFrom
			t.cooldown = 10
		}
		t.trialFrom = 0
		return t.limit
	}
	if t.cooldown > 0 {
		t.cooldown--
		return t.limit
	}
	if rate > 0 && t.limit < t.maximum {
		t.baseline, t.trialFrom = rate, t.limit
		t.limit = min(t.limit*2, t.maximum)
	}
	return t.limit
}

func (e *Engine) adaptiveParallel(ctx context.Context, url string, meta metadata, f *os.File, s *scheduler) error {
	poolCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() { stop(); wg.Wait() }()
	tuner := connectionTuner{limit: min(4, e.cfg.Workers), maximum: e.cfg.Workers}
	type worker struct {
		cancel   context.CancelFunc
		retiring bool
	}
	type completion struct {
		id    int
		chunk *chunk
		err   error
	}
	workers := make(map[int]*worker)
	done := make(chan completion, e.cfg.Workers)
	nextID := 0
	fill := func() {
		for len(workers) < tuner.limit && poolCtx.Err() == nil {
			c := s.acquire()
			if c == nil {
				break
			}
			id := nextID
			nextID++
			workerCtx, cancel := context.WithCancel(poolCtx)
			workers[id] = &worker{cancel: cancel}
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := e.downloadChunk(workerCtx, url, meta, f, s, c, make([]byte, e.cfg.BufferSize))
				done <- completion{id, c, err}
			}()
		}
	}
	fill()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	previous, previousRetries := s.counters()
	last := time.Now()
	for len(workers) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-done:
			w := workers[result.id]
			w.cancel()
			delete(workers, result.id)
			if result.err != nil && !w.retiring {
				return result.err
			}
			if result.err == nil {
				s.mark(result.chunk, "complete", false)
			} else {
				s.release(result.chunk)
			}
			fill()
		case now := <-ticker.C:
			written, retries := s.counters()
			rate := float64(written-previous) / now.Sub(last).Seconds()
			retryDelta := retries - previousRetries
			previous, previousRetries, last = written, retries, now
			if e.cfg.Control.IsPaused() {
				tuner.trialFrom = 0
				tuner.cooldown = 1
				continue
			}
			target := tuner.sample(rate, retryDelta)
			s.setLimit(target)
			active := 0
			for _, w := range workers {
				if !w.retiring {
					active++
				}
			}
			for _, w := range workers {
				if active <= target {
					break
				}
				if !w.retiring {
					w.retiring = true
					w.cancel()
					active--
				}
			}
			fill()
		}
	}
	return ctx.Err()
}
