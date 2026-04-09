package appcore

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const serviceActionTimeout = 30 * time.Second

var scStateMatcher = regexp.MustCompile(`STATE\s*:\s*\d+\s+([A-Z_]+)`)

func extractSCState(output string) (string, error) {
	matches := scStateMatcher.FindStringSubmatch(strings.ToUpper(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("field STATE tidak ditemukan")
	}
	return strings.TrimSpace(matches[1]), nil
}

func queryServiceState(name string) (string, error) {
	out, err := commandCombinedOutput("sc", "query", name)
	text := string(out)
	if err != nil {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "1060") || strings.Contains(lower, "does not exist") || strings.Contains(lower, "tidak ada") {
			return "not_found", nil
		}
	}

	scState, parseErr := extractSCState(text)
	if parseErr != nil {
		return "unknown", fmt.Errorf("gagal query service %s: %s", name, strings.TrimSpace(text))
	}

	switch scState {
	case "RUNNING":
		return "running", nil
	case "STOPPED":
		return "stopped", nil
	case "PAUSED":
		return "paused", nil
	case "START_PENDING":
		return "start_pending", nil
	case "STOP_PENDING":
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
	return waitServiceState(name, "running", serviceActionTimeout)
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
	return waitServiceState(name, "stopped", serviceActionTimeout)
}

func restartService(name string) error {
	if err := stopService(name); err != nil {
		return err
	}
	return startService(name)
}

func waitServiceState(name, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastState := "unknown"
	lastErr := ""
	for time.Now().Before(deadline) {
		state, err := queryServiceState(name)
		if err == nil {
			lastState = state
		} else {
			lastErr = err.Error()
		}
		if err == nil && state == target {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	if strings.TrimSpace(lastErr) != "" {
		return fmt.Errorf("timeout menunggu service %s ke state %s (state terakhir: %s, error terakhir: %s)", name, target, lastState, lastErr)
	}
	return fmt.Errorf("timeout menunggu service %s ke state %s (state terakhir: %s)", name, target, lastState)
}
