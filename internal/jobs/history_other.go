//go:build !windows

package jobs

import "os"

func replaceHistory(from, to string) error { return os.Rename(from, to) }
