package appcore

import (
	"fmt"
	"strings"
	"time"
)

func queryServiceState(name string) (string, error) {
	out, err := commandCombinedOutput("sc", "query", name)
	text := strings.ToUpper(string(out))
	if err != nil && !strings.Contains(text, "STATE") {
		return "unknown", fmt.Errorf("gagal query service %s: %s", name, strings.TrimSpace(string(out)))
	}

	switch {
	case strings.Contains(text, "RUNNING"):
		return "running", nil
	case strings.Contains(text, "STOPPED"):
		return "stopped", nil
	case strings.Contains(text, "PAUSED"):
		return "paused", nil
	case strings.Contains(text, "START_PENDING"):
		return "start_pending", nil
	case strings.Contains(text, "STOP_PENDING"):
		return "stop_pending", nil
	default:
		return "unknown", nil
	}
}

func startService(name string) error {
	out, err := commandCombinedOutput("sc", "start", name)
	if err != nil {
		return fmt.Errorf("gagal start service %s: %s", name, strings.TrimSpace(string(out)))
	}
	return waitServiceState(name, "running", 20*time.Second)
}

func stopService(name string) error {
	out, err := commandCombinedOutput("sc", "stop", name)
	if err != nil {
		text := strings.ToLower(string(out))
		// If already stopped keep behavior idempotent.
		if !strings.Contains(text, "service has not been started") {
			return fmt.Errorf("gagal stop service %s: %s", name, strings.TrimSpace(string(out)))
		}
	}
	return waitServiceState(name, "stopped", 20*time.Second)
}

func restartService(name string) error {
	if err := stopService(name); err != nil {
		return err
	}
	return startService(name)
}

func waitServiceState(name, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := queryServiceState(name)
		if err == nil && state == target {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout menunggu service %s ke state %s", name, target)
}
