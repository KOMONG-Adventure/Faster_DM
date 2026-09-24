//go:build !windows

package main

import "errors"

func launchUpdateInstaller(string) error {
	return errors.New("Шууд суулгах нь зөвхөн Windows дээр дэмжигдэнэ.")
}
