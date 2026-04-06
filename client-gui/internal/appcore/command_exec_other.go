//go:build !windows
// +build !windows

package appcore

import "os/exec"

func commandCombinedOutput(name string, arg ...string) ([]byte, error) {
	return exec.Command(name, arg...).CombinedOutput()
}