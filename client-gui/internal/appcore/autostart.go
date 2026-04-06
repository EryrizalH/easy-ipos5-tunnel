package appcore

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func EnableTaskSchedulerAutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	tr := fmt.Sprintf("\"%s\" --hidden", exePath)
	cmd := exec.Command(
		"schtasks",
		"/Create",
		"/TN", taskName,
		"/TR", tr,
		"/SC", "ONLOGON",
		"/F",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gagal membuat task autostart: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func DisableTaskSchedulerAutoStart() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If task doesn't exist, treat as success.
		text := strings.ToLower(string(out))
		if !strings.Contains(text, "cannot find the file specified") &&
			!strings.Contains(text, "the system cannot find the file specified") {
			return fmt.Errorf("gagal menghapus task autostart: %s", strings.TrimSpace(string(out)))
		}
	}
	return nil
}
