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
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibWinPth))
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
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibWinPth))
	mustWrite(t, filepath.Join(pgBinDir, "psql.exe"))

	cfg := Config{PGBinPath: pgBinDir}
	paths := BundlePaths{PgBouncerPath: filepath.Join(bundleDir, pgBouncerBinary)}

	err := preflightPgBouncerInstallWithChecks(cfg, paths,
		func(address string, _ time.Duration) error {
			if !strings.Contains(address, ":5445") {
				t.Fatalf("expected preflight backend address to use 5445, got %s", address)
			}
			return errors.New("dial tcp timeout")
		},
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
	mustWrite(t, filepath.Join(bundleDir, pgBouncerLibWinPth))
	mustWrite(t, filepath.Join(pgBinDir, "psql.exe"))

	cfg := Config{PGBinPath: pgBinDir}
	paths := BundlePaths{PgBouncerPath: filepath.Join(bundleDir, pgBouncerBinary)}

	err := preflightPgBouncerInstallWithChecks(cfg, paths,
		func(_ string, _ time.Duration) error { return nil },
		func(address string) error {
			if address != "0.0.0.0:5444" {
				t.Fatalf("expected PgBouncer listener preflight on 0.0.0.0:5444, got %s", address)
			}
			return fmt.Errorf("listen %s: address already in use", address)
		},
		func(_ string) (bool, error) { return false, nil },
	)
	if err == nil {
		t.Fatal("expected port unavailable error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "port listen") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestChangePostgresPortIfNeededWithHooks_NoOpWhenAlreadyTarget(t *testing.T) {
	runCalled := false
	restartCalled := false
	reachableCalled := false

	err := changePostgresPortIfNeededWithHooks(
		`D:\pgbin`,
		postgresLegacyPort,
		func(_ string) (int, error) { return postgresLegacyPort, nil },
		func(_ string, _ int, _ string, _ string) error {
			runCalled = true
			return nil
		},
		func() error {
			restartCalled = true
			return nil
		},
		func(_ string, _ time.Duration) error {
			reachableCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("changePostgresPortIfNeededWithHooks() unexpected error = %v", err)
	}
	if runCalled || restartCalled || reachableCalled {
		t.Fatalf("expected no-op when port already target; got run=%v restart=%v reachable=%v", runCalled, restartCalled, reachableCalled)
	}
}

func TestChangePostgresPortIfNeededWithHooks_RollbackTo5444(t *testing.T) {
	calledSQL := ""
	restartCalled := false
	reachableAddr := ""

	err := changePostgresPortIfNeededWithHooks(
		`D:\pgbin`,
		postgresLegacyPort,
		func(_ string) (int, error) { return postgresBackendPort, nil },
		func(_ string, port int, databaseName, sql string) error {
			if port != postgresBackendPort {
				t.Fatalf("expected ALTER SYSTEM to run on current port %d, got %d", postgresBackendPort, port)
			}
			if databaseName != "postgres" {
				t.Fatalf("expected database postgres, got %s", databaseName)
			}
			calledSQL = sql
			return nil
		},
		func() error {
			restartCalled = true
			return nil
		},
		func(address string, _ time.Duration) error {
			reachableAddr = address
			return nil
		},
	)
	if err != nil {
		t.Fatalf("changePostgresPortIfNeededWithHooks() unexpected error = %v", err)
	}
	if calledSQL != "ALTER SYSTEM SET port = 5444;" {
		t.Fatalf("unexpected SQL: %s", calledSQL)
	}
	if !restartCalled {
		t.Fatal("expected PostgreSQL restart to be called")
	}
	if reachableAddr != "127.0.0.1:5444" {
		t.Fatalf("expected verify address 127.0.0.1:5444, got %s", reachableAddr)
	}
}

func TestRunPostgresCommandWithFallbackHooks_UsesWorkaroundForAlterSystemSuperuserError(t *testing.T) {
	primaryCalled := false
	fallbackCalled := false

	err := runPostgresCommandWithFallbackHooks(
		`D:\pgbin`,
		postgresLegacyPort,
		"postgres",
		"ALTER SYSTEM SET port = 5445;",
		func(_ string, _ int, _ string, _ string) error {
			primaryCalled = true
			return errors.New("psql command gagal: exit status 1: ERROR: must be superuser to execute ALTER SYSTEM command")
		},
		func(_ string, databaseName, sql string) error {
			fallbackCalled = true
			if databaseName != "postgres" {
				t.Fatalf("expected database postgres, got %s", databaseName)
			}
			if sql != "ALTER SYSTEM SET port = 5445;" {
				t.Fatalf("unexpected SQL passed to fallback: %s", sql)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runPostgresCommandWithFallbackHooks() unexpected error = %v", err)
	}
	if !primaryCalled {
		t.Fatal("expected primary command to be attempted first")
	}
	if !fallbackCalled {
		t.Fatal("expected fallback workaround to be called")
	}
}

func TestRunPostgresCommandWithFallbackHooks_ReturnsOriginalErrorWhenNotSuperuserFailure(t *testing.T) {
	expectedErr := errors.New("psql command gagal: connection refused")

	err := runPostgresCommandWithFallbackHooks(
		`D:\pgbin`,
		postgresLegacyPort,
		"postgres",
		"SELECT 1;",
		func(_ string, _ int, _ string, _ string) error {
			return expectedErr
		},
		func(_ string, _ string, _ string) error {
			t.Fatal("fallback should not be called for non-superuser errors")
			return nil
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected original error, got %v", err)
	}
}
