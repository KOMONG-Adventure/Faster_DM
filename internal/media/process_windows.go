package media

import (
	"os/exec"
	"strconv"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if cmd.Cancel != nil {
		cmd.Cancel = func() error {
			kill := exec.Command("taskkill.exe", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
			kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			return kill.Run()
		}
	}
}
