//go:build !windows && !linux

package download

import "os"

// Бусад OS дээр логик хэмжээ тогтооно; физик allocation нь баталгаагүй.
func allocate(f *os.File, size int64) error { return f.Truncate(size) }
