package download

import (
	"os"
	"runtime"
	"syscall"
)

func allocate(f *os.File, size int64) error {
	if size > 0 {
		var err error
		for {
			err = syscall.Fallocate(int(f.Fd()), 0, 0, size)
			if err != syscall.EINTR {
				break
			}
		}
		runtime.KeepAlive(f)
		if err != nil {
			return os.NewSyscallError("fallocate", err)
		}
	}
	return f.Truncate(size)
}
