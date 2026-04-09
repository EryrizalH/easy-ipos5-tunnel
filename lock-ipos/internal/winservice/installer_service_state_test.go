package winservice

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractSCState_Success(t *testing.T) {
	out := "STATE              : 4  RUNNING"
	state, err := extractSCState(out)
	if err != nil {
		t.Fatalf("extractSCState() error = %v", err)
	}
	if state != "RUNNING" {
		t.Fatalf("expected RUNNING, got %s", state)
	}
}

func TestExtractSCState_FieldNotFound(t *testing.T) {
	_, err := extractSCState("SERVICE_NAME: PgBouncer")
	if err == nil {
		t.Fatal("expected error when STATE field is missing")
	}
}

func TestWaitServiceStateWithQuery_SuccessAfterPending(t *testing.T) {
	calls := 0
	queryFn := func(_ string) (string, string, error) {
		calls++
		if calls < 3 {
			return "START_PENDING", "STATE : 2 START_PENDING", nil
		}
		return "RUNNING", "STATE : 4 RUNNING", nil
	}

	err := waitServiceStateWithQuery("PgBouncer", "RUNNING", 50*time.Millisecond, 0, queryFn, func(time.Duration) {})
	if err != nil {
		t.Fatalf("waitServiceStateWithQuery() error = %v", err)
	}
	if calls < 3 {
		t.Fatalf("expected at least 3 polls, got %d", calls)
	}
}

func TestWaitServiceStateWithQuery_TimeoutIncludesLastState(t *testing.T) {
	queryFn := func(_ string) (string, string, error) {
		return "START_PENDING", "STATE : 2 START_PENDING", nil
	}

	err := waitServiceStateWithQuery("PgBouncer", "RUNNING", 5*time.Millisecond, 0, queryFn, func(time.Duration) {})
	if err == nil {
		t.Fatal("expected timeout error")
	}

	msg := strings.ToUpper(err.Error())
	if !strings.Contains(msg, "START_PENDING") {
		t.Fatalf("timeout error should include last state, got: %v", err)
	}
}

func TestPreflightPgBouncerInstallWithChecks_Success(t *testing.T) {
	tmp := t.TempDir()
	bundleDir := filepath.Join(tmp, "bundle")
	pgBinDir := filepath.Join(tmp, "pgbin")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}
	if err := os.MkdirAll(pgBinDir, 0o755); err != nil {
		t.Fatalf("failed to create pgbin dir: %v", err)
	}

	mustWrite(t, filepath.Join(bundleDir, pgBouncerBinary))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibEvent))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibSSL))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibCrypto))
	mustWrite(t, filepath.Join(pgBinDir, "psql.exe"))

	cfg := Config{PGBinPath: pgBinDir}
	paths := BundlePaths{PgBouncerPath: filepath.Join(bundleDir, pgBouncerBinary)}

	err := preflightPgBouncerInstallWithChecks(cfg, paths,
		func(_ string, _ time.Duration) error { return nil },
		func(_ string) error { return nil },
		func(_ string) (bool, error) { return false, nil },
	)
	if err != nil {
		t.Fatalf("preflight should succeed, got: %v", err)
	}
}

func TestPreflightPgBouncerInstallWithChecks_BackendUnreachable(t *testing.T) {
	tmp := t.TempDir()
	bundleDir := filepath.Join(tmp, "bundle")
	pgBinDir := filepath.Join(tmp, "pgbin")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}
	if err := os.MkdirAll(pgBinDir, 0o755); err != nil {
		t.Fatalf("failed to create pgbin dir: %v", err)
	}

	mustWrite(t, filepath.Join(bundleDir, pgBouncerBinary))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibEvent))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibSSL))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibCrypto))
	mustWrite(t, filepath.Join(pgBinDir, "psql.exe"))

	cfg := Config{PGBinPath: pgBinDir}
	paths := BundlePaths{PgBouncerPath: filepath.Join(bundleDir, pgBouncerBinary)}

	err := preflightPgBouncerInstallWithChecks(cfg, paths,
		func(_ string, _ time.Duration) error { return errors.New("dial tcp timeout") },
		func(_ string) error { return nil },
		func(_ string) (bool, error) { return false, nil },
	)
	if err == nil {
		t.Fatal("expected backend unreachable error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "backend postgresql") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPreflightPgBouncerInstallWithChecks_PortUnavailable(t *testing.T) {
	tmp := t.TempDir()
	bundleDir := filepath.Join(tmp, "bundle")
	pgBinDir := filepath.Join(tmp, "pgbin")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}
	if err := os.MkdirAll(pgBinDir, 0o755); err != nil {
		t.Fatalf("failed to create pgbin dir: %v", err)
	}

	mustWrite(t, filepath.Join(bundleDir, pgBouncerBinary))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibEvent))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibSSL))
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibCrypto))
	mustWrite(t, filepath.Join(pgBinDir, "psql.exe"))

	cfg := Config{PGBinPath: pgBinDir}
	paths := BundlePaths{PgBouncerPath: filepath.Join(bundleDir, pgBouncerBinary)}

	err := preflightPgBouncerInstallWithChecks(cfg, paths,
		func(_ string, _ time.Duration) error { return nil },
		func(address string) error { return fmt.Errorf("listen %s: address already in use", address) },
		func(_ string) (bool, error) { return false, nil },
	)
	if err == nil {
		t.Fatal("expected port unavailable error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "port listen") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
