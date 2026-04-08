package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func handleFatalStartupError(err error) {
	if err == nil {
		return
	}

	logPath := writeStartupErrorLog(err)
	message := "Easy Rathole Client GUI gagal dijalankan.\n\n" + err.Error()
	if logPath != "" {
		message += "\n\nLog: " + logPath
	}
	message += "\n\nPastikan Microsoft Edge WebView2 Runtime sudah terpasang."
	showFatalDialog("Easy Rathole Client GUI", message)
	os.Exit(1)
}

func writeStartupErrorLog(err error) string {
	logDir := resolveLogDir()
	if logDir == "" {
		return ""
	}
	if mkErr := os.MkdirAll(logDir, 0o755); mkErr != nil {
		return ""
	}

	logPath := filepath.Join(logDir, "startup.log")
	entry := fmt.Sprintf(
		"[%s] startup error\nos=%s arch=%s\ngo=%s\nerror=%v\n\n",
		time.Now().Format(time.RFC3339),
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version(),
		err,
	)

	f, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if openErr != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	if _, writeErr := f.WriteString(entry); writeErr != nil {
		return ""
	}
	return logPath
}

func resolveLogDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "easy-rathole-client-gui", "logs")
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfgDir, "easy-rathole-client-gui", "logs")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".easy-rathole-client-gui", "logs")
	}
	if tmp := os.TempDir(); tmp != "" {
		return filepath.Join(tmp, "easy-rathole-client-gui", "logs")
	}
	return ""
}
