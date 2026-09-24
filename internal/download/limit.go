package download

import (
	"context"
	"sync"
	"time"
)

// Limiter is shared by all connections of one download.
type Limiter struct {
	mu             sync.Mutex
	next           time.Time
	BytesPerSecond int64
}

func (l *Limiter) Wait(ctx context.Context, n int) error {
	if l == nil || l.BytesPerSecond <= 0 {
		return ctx.Err()
	}
	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	l.next = l.next.Add(time.Duration(float64(n) / float64(l.BytesPerSecond) * float64(time.Second)))
	delay := time.Until(l.next)
	l.mu.Unlock()
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (e *Engine) limit(ctx context.Context, body interface{}, n int) error {
	if e.cfg.Limiter == nil {
		return nil
	}
	// Intentional throttling is not a network idle timeout.
	b, ok := body.(*idleBody)
	if ok {
		b.timer.Stop()
		ctx = b.pauseContext
	}
	err := e.cfg.Limiter.Wait(ctx, n)
	if ok {
		b.timer.Reset(b.timeout)
		if context.Cause(b.pauseContext) == errPaused {
			return errPaused
		}
	}
	return err
}
