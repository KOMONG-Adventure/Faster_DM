//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func launchUpdateInstaller(path string) error {
	cmd := exec.Command(path, "/SILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/NOCLOSEAPPLICATIONS", "/UPDATEPID="+strconv.Itoa(os.Getpid()), "/LOG="+filepath.Join(filepath.Dir(path), "install.log"))
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
