package download

import (
	"context"
	"os"
	"time"
)

// Wait for all writers to exit before releasing ranges and reducing concurrency.
func (e *Engine) parallelWithRecovery(ctx context.Context, url string, meta metadata, f *os.File, s *scheduler, workers int, adaptive bool) error {
	for attempt := 0; ; attempt++ {
		var err error
		if adaptive {
			err = e.adaptiveParallel(ctx, url, meta, f, s)
		} else {
			err = e.parallel(ctx, url, meta, f, s, workers)
		}
		if err == nil || ctx.Err() != nil || !overloaded(err) || attempt >= e.cfg.MaxRetries {
			return err
		}
		s.mu.Lock()
		workers = max(1, s.connectionLimit/2)
		for _, c := range s.chunks {
			c.active = false
			c.reserved = c.next
			c.status = "retrying"
			if c.next == c.end {
				c.status = "complete"
			}
		}
		s.connectionLimit = workers
		s.retryAt = time.Now().Add(retryDelay(attempt, err))
		s.message = "Сервер ачаалалтай. Холболтыг бууруулж дахин оролдоно."
		delay := time.Until(s.retryAt)
		s.mu.Unlock()
		if err = waitDelay(ctx, delay); err != nil {
			return err
		}
		s.mu.Lock()
		s.retryAt = time.Time{}
		s.message = ""
		s.mu.Unlock()
		adaptive = false
	}
}
