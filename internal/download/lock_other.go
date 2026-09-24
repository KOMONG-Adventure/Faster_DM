//go:build !windows

package download

import (
	"golang.org/x/sys/unix"
	"os"
)

func LockPartial(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
