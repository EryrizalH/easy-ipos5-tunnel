//go:build windows
// +build windows

package appcore

import (
	"os/exec"
	"syscall"
)

func commandCombinedOutput(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	return cmd.CombinedOutput()
}