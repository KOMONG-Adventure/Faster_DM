package download

import (
	"sync"
	"time"
)

type chunk struct {
	id                         int
	start, next, end, reserved int64
	active                     bool
	status                     string
	retries                    int
}

type scheduler struct {
	mu       sync.Mutex
	chunks   []*chunk
	min      int64
	total    int64
	previous map[int]int64
	last     time.Time
}

func newScheduler(size int64, workers int, min int64) *scheduler {
	s := &scheduler{min: min, total: size, previous: make(map[int]int64), last: time.Now()}
	n := workers
	if size/min < int64(n) {
		n = int(size / min)
	}
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		// Үржвэрийн int64 overflow-оос зайлсхийсэн тэнцүү хуваарилалт.
		start := size / int64(n) * int64(i)
		end := size / int64(n) * int64(i+1)
		if i == n-1 {
			end = size
		}
		s.chunks = append(s.chunks, &chunk{id: i, start: start, next: start, reserved: start, end: end, status: "queued"})
	}
	return s
}

func (s *scheduler) acquire() *chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.chunks {
		if !c.active && c.next < c.end {
			c.active, c.status = true, "downloading"
			return c
		}
	}
	// Зөвхөн унших/бичихээр нөөцлөөгүй сүүлийг хуваана.
	// Энэ түгжээ нь disk I/O-г хамрахгүй: worker-ууд зэрэг WriteAt хийж чадна.
	var victim *chunk
	for _, c := range s.chunks {
		if c.active && c.end-c.reserved >= 2*s.min && (victim == nil || c.end-c.reserved > victim.end-victim.reserved) {
			victim = c
		}
	}
	// Урт таталтын UI snapshot болон scheduler-ийн санах ойг хязгаарлана.
	if victim == nil || len(s.chunks) >= 4096 {
		return nil
	}
	mid := victim.reserved + (victim.end-victim.reserved)/2
	c := &chunk{id: len(s.chunks), start: mid, next: mid, reserved: mid, end: victim.end, active: true, status: "downloading"}
	victim.end = mid
	s.chunks = append(s.chunks, c)
	return c
}

func (s *scheduler) bounds(c *chunk) (int64, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return c.next, c.end
}

func (s *scheduler) reserve(c *chunk, capacity int) (int64, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := int64(capacity)
	if c.end-c.next < n {
		n = c.end - c.next
	}
	c.reserved = c.next + n
	return c.next, int(n)
}

func (s *scheduler) advance(c *chunk, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.next += int64(n)
	c.reserved = c.next
}

func (s *scheduler) mark(c *chunk, status string, retry bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.status = status
	if retry {
		c.retries++
	}
	if status == "complete" {
		c.active = false
	}
}

func (s *scheduler) snapshot(status string) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	dt := now.Sub(s.last).Seconds()
	r := Snapshot{Total: s.total, Status: status, Chunks: make([]ChunkSnapshot, 0, len(s.chunks))}
	for _, c := range s.chunks {
		done := c.next - c.start
		speed := float64(done-s.previous[c.id]) / dt
		state := c.status
		if status != "downloading" && state != "complete" {
			state = status
		}
		r.Chunks = append(r.Chunks, ChunkSnapshot{ID: c.id, Start: c.start, End: c.end, Downloaded: done, BytesPerSecond: speed, Status: state, Retries: c.retries})
		r.Downloaded += done
		r.BytesPerSecond += speed
		s.previous[c.id] = done
	}
	s.last = now
	return r
}
