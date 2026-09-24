package download

import (
	"context"
	"errors"
	"sync"
)

var errPaused = errors.New("таталтыг түр зогсоосон")

// Control нь нэг таталтыг цуцлахгүйгээр түр зогсооно. Engine ба файл амьд үлдэнэ.
type Control struct {
	mu       sync.Mutex
	paused   bool
	changed  chan struct{}
	requests map[uint64]context.CancelCauseFunc
	sequence uint64
}

func NewControl() *Control {
	return &Control{changed: make(chan struct{}), requests: make(map[uint64]context.CancelCauseFunc)}
}

func (c *Control) IsPaused() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// RequestContext нь гадаад таталтын процессод pause-cancel холбоос өгнө.
func (c *Control) RequestContext(ctx context.Context) (context.Context, func(), error) {
	return c.request(ctx)
}
func PausedContext(ctx context.Context) bool { return errors.Is(context.Cause(ctx), errPaused) }
func (c *Control) Pause() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return
	}
	c.paused = true
	close(c.changed)
	c.changed = make(chan struct{})
	for _, cancel := range c.requests {
		cancel(errPaused)
	}
}
func (c *Control) Resume() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		return
	}
	c.paused = false
	close(c.changed)
	c.changed = make(chan struct{})
}
func (c *Control) Wait(ctx context.Context) error {
	if c == nil {
		return ctx.Err()
	}
	for {
		c.mu.Lock()
		paused, changed := c.paused, c.changed
		c.mu.Unlock()
		if !paused {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}
func (c *Control) request(ctx context.Context) (context.Context, func(), error) {
	if c == nil {
		return ctx, func() {}, ctx.Err()
	}
	for {
		if err := c.Wait(ctx); err != nil {
			return nil, nil, err
		}
		c.mu.Lock()
		if c.paused {
			c.mu.Unlock()
			continue
		}
		rctx, cancel := context.WithCancelCause(ctx)
		c.sequence++
		id := c.sequence
		c.requests[id] = cancel
		c.mu.Unlock()
		return rctx, func() { c.mu.Lock(); delete(c.requests, id); c.mu.Unlock(); cancel(context.Canceled) }, nil
	}
}
