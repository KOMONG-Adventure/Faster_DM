package diskspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const Reserve = int64(64 << 20)

var ErrLow = errors.New("дискний сул зай хүрэлцэхгүй")

func Free(path string) (int64, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	for {
		info, err := os.Stat(p)
		if err == nil {
			if !info.IsDir() {
				p = filepath.Dir(p)
			}
			return available(p)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return 0, err
		}
		p = parent
	}
}
func Check(path string, needed int64) error {
	free, err := Free(path)
	if err != nil {
		return fmt.Errorf("дискний зай шалгах: %w", err)
	}
	if !Enough(free, needed) {
		return fmt.Errorf("%w: сул %.1f MB, нэмэлт шаардлага %.1f MB (+67.1 MB нөөц). Зай гаргаад үргэлжлүүлнэ үү", ErrLow, float64(free)/1e6, float64(max(needed, 0))/1e6)
	}
	return nil
}
func Enough(free, needed int64) bool { return free >= Reserve && max(needed, 0) <= free-Reserve }
func IsFull(err error) bool          { return errors.Is(err, ErrLow) || isFull(err) }
