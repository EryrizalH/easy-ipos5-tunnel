package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupLogFallsBackAndPersistsFullError(t *testing.T) {
	Close()
	t.Cleanup(Close)
	directory := t.TempDir()
	blocked := filepath.Join(directory, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(directory, "logs", "setup.log")
	if err := initAtPaths([]string{filepath.Join(blocked, "log.txt"), fallback}); err != nil {
		t.Fatal(err)
	}
	if got := Path(); got != fallback {
		t.Fatalf("log path = %q, want %q", got, fallback)
	}
	message := "identitas PostgreSQL iPgSql.9.5 tidak pasti: proses postmaster tidak dapat diperiksa: Access is denied."
	Error(message)
	raw, err := os.ReadFile(fallback)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "[ERROR] "+message) {
		t.Fatalf("full error missing from log: %s", raw)
	}
	Close()
	if Path() != "" {
		t.Fatal("closed logger must not advertise an available log")
	}
}

func TestSetupLogReportsUnavailableLocation(t *testing.T) {
	Close()
	directory := t.TempDir()
	if err := initAtPaths([]string{directory}); err == nil {
		t.Fatal("expected an error when log path is a directory")
	}
	if Path() != "" {
		t.Fatal("unavailable logger must not advertise a log path")
	}
}
