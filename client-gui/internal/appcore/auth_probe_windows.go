//go:build windows
// +build windows

package appcore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var recentAuthFailurePatterns = []string{
	"auth failed",
	"authentication failed",
	"invalid token",
	"unauthorized",
	"forbidden",
}

func detectRecentAuthFailure(serviceName string) (bool, error) {
	logPath, err := serviceLogPath(serviceName)
	if err != nil {
		return false, err
	}

	// Primary signal: dedicated stderr log if NSSM app logging is enabled.
	if fromFile, err := detectAuthFailureFromFile(logPath); err == nil && fromFile {
		return true, nil
	}

	// Fallback signal: Windows Application event log for service/provider entries.
	return detectAuthFailureFromEventLog(serviceName)
}

func detectAuthFailureFromFile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	text := strings.ToLower(string(data))
	return hasAuthFailureKeyword(text), nil
}

func detectAuthFailureFromEventLog(serviceName string) (bool, error) {
	pattern := strings.Join(recentAuthFailurePatterns, "|")
	script := strings.Join([]string{
		"$ErrorActionPreference='Stop'",
		"$start=(Get-Date).AddMinutes(-15)",
		"$events=Get-WinEvent -FilterHashtable @{LogName='Application'; StartTime=$start} -MaxEvents 300 -ErrorAction SilentlyContinue",
		"$match=$events | Where-Object {",
		"  ($_.ProviderName -match 'nssm|rathole|easyrathole|ipos5tunnelpublik' -or $_.Message -match 'rathole|easyrathole|ipos5') -and",
		"  $_.Message -match '" + pattern + "'",
		"} | Select-Object -First 1 -ExpandProperty Message",
		"if ($match) { 'AUTH_FAIL' }",
	}, "; ")
	out, err := commandCombinedOutput("powershell.exe", "-NoProfile", "-Command", script)
	if err != nil {
		return false, nil
	}
	return strings.Contains(strings.ToUpper(string(out)), "AUTH_FAIL"), nil
}

func hasAuthFailureKeyword(text string) bool {
	for _, keyword := range recentAuthFailurePatterns {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func serviceLogPath(serviceName string) (string, error) {
	programData := os.Getenv("ProgramData")
	if strings.TrimSpace(programData) == "" {
		return "", errors.New("env ProgramData tidak ditemukan")
	}
	safeService := strings.TrimSpace(serviceName)
	if safeService == "" {
		safeService = defaultServiceName
	}
	return filepath.Join(programData, "easy-rathole-client", "logs", safeService+".stderr.log"), nil
}
