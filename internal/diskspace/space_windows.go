package diskspace

import (
	"errors"
	"golang.org/x/sys/windows"
)

func available(path string) (int64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free uint64
	err = windows.GetDiskFreeSpaceEx(p, &free, nil, nil)
	return int64(min(free, uint64(1<<63-1))), err
}
func isFull(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) || errors.Is(err, windows.ERROR_HANDLE_DISK_FULL)
}
