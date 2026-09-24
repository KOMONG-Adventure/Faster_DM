package download

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type savedChunk struct{ Start, Next, End int64 }
type checkpoint struct {
	Version   int
	URL, ETag string
	Size      int64
	Chunks    []savedChunk
}

// Snapshot offsets first, then flush the file before committing metadata.
// Writes after the snapshot are safely downloaded again after a crash.
func saveCheckpoint(path, url string, meta metadata, f *os.File, s *scheduler) error {
	c := checkpoint{Version: 1, URL: url, ETag: meta.etag, Size: meta.size}
	s.mu.Lock()
	for _, p := range s.chunks {
		c.Chunks = append(c.Chunks, savedChunk{p.start, p.next, p.end})
	}
	s.mu.Unlock()
	if err := f.Sync(); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".checkpoint-*")
	if err != nil {
		return err
	}
	defer func() { tmp.Close(); os.Remove(tmp.Name()) }()
	if err = tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return replaceCheckpoint(tmp.Name(), path)
}

func restoreCheckpoint(path, url string, meta metadata, f *os.File, s *scheduler) (bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	var c checkpoint
	if err = json.NewDecoder(io.LimitReader(file, 2<<20)).Decode(&c); err != nil {
		return false, fmt.Errorf("үргэлжлүүлэлтийн мэдээлэл гэмтсэн: %w", err)
	}
	if c.Version != 1 || c.URL != url || c.ETag != meta.etag || c.Size != meta.size || !meta.parallel {
		return false, ErrChanged
	}
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() != c.Size || len(c.Chunks) == 0 || len(c.Chunks) > 4096 {
		return false, ErrProtocol
	}
	// Work stealing appends chunks, so validate coverage in offset order.
	sort.Slice(c.Chunks, func(i, j int) bool { return c.Chunks[i].Start < c.Chunks[j].Start })
	var end int64
	for _, p := range c.Chunks {
		if p.Start != end || p.Next < p.Start || p.Next > p.End || p.End < p.Start || p.End > c.Size {
			return false, ErrProtocol
		}
		end = p.End
	}
	if end != c.Size {
		return false, ErrProtocol
	}
	s.chunks = nil
	for i, p := range c.Chunks {
		status := "queued"
		if p.Next == p.End {
			status = "complete"
		}
		s.chunks = append(s.chunks, &chunk{id: i, start: p.Start, next: p.Next, reserved: p.Next, end: p.End, status: status})
		s.previous[i] = p.Next - p.Start
	}
	return true, nil
}
