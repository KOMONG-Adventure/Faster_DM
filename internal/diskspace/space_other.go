//go:build !windows

package diskspace

import (
	"errors"
	"golang.org/x/sys/unix"
)

func available(path string) (int64, error) {
	var s unix.Statfs_t
	err := unix.Statfs(path, &s)
	if err != nil {
		return 0, err
	}
	return int64(s.Bavail) * int64(s.Bsize), nil
}
func isFull(err error) bool { return errors.Is(err, unix.ENOSPC) || errors.Is(err, unix.EDQUOT) }
