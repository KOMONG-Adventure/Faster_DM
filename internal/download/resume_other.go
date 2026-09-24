//go:build !windows

package download

import "os"

func replaceCheckpoint(from, to string) error { return os.Rename(from, to) }
