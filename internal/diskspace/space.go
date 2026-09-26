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
		return fmt.Errorf("%w: сул %.1f MiB, нэмэлт шаардлага %.1f MiB (+64 MiB нөөц). Зай гаргаад үргэлжлүүлнэ үү", ErrLow, float64(free)/(1<<20), float64(max(needed, 0))/(1<<20))
	}
	return nil
}
func Enough(free, needed int64) bool { return free >= Reserve && max(needed, 0) <= free-Reserve }
func IsFull(err error) bool          { return errors.Is(err, ErrLow) || isFull(err) }
