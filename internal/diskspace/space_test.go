package diskspace

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestCapacityBoundaries(t *testing.T) {
	for _, c := range []struct {
		free, need int64
		want       bool
	}{{Reserve, 0, true}, {Reserve - 1, 0, false}, {Reserve + 100, 100, true}, {Reserve + 99, 100, false}, {1<<63 - 1, 1<<63 - 1, false}, {Reserve, -1, true}} {
		if Enough(c.free, c.need) != c.want {
			t.Fatalf("%+v", c)
		}
	}
	if !IsFull(fmt.Errorf("write: %w", ErrLow)) {
		t.Fatal("wrapped low space not recognized")
	}
	if free, err := Free(filepath.Join(t.TempDir(), "missing", "nested")); err != nil || free <= 0 {
		t.Fatalf("free=%d err=%v", free, err)
	}
}
