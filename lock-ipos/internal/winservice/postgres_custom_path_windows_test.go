//go:build windows

package winservice

import (
	"errors"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestPostmasterOptionsCustomLocationWithSpaces(t *testing.T) {
	expected := `E:\Projects\ipos\Server System 1.0\pgsql9.5\bin\postgres.exe`
	directory := `E:\Projects\ipos\Server System 1.0\pgsql9.5\data`
	for _, line := range []string{
		`E:/Projects/ipos/Server System 1.0/pgsql9.5/bin/postgres.exe "-D" "E:\Projects\ipos\Server System 1.0\pgsql9.5\data"` + "\r\n",
		`"E:\Projects\ipos\Server System 1.0\pgsql9.5\bin\postgres.exe" "-D" "E:\Projects\ipos\Server System 1.0\pgsql9.5\data"`,
		`e:/projects/IPOS/Server System 1.0/pgsql9.5/bin/POSTGRES.EXE "-D" "E:\Projects\ipos\Server System 1.0\pgsql9.5\data"`,
	} {
		executable, args, err := postgresPostmasterArguments(line, expected)
		if err != nil || !samePostgresPath(executable, expected) || !reflect.DeepEqual(args, []string{"-D", directory}) {
			t.Fatalf("custom postmaster.opts failed: executable=%q args=%q err=%v", executable, args, err)
		}
	}
}

func TestPostmasterOptionsRejectWrongInstanceOrUnsafeArguments(t *testing.T) {
	expected := `E:\Projects\ipos\Server System 1.0\pgsql9.5\bin\postgres.exe`
	for _, line := range []string{
		`E:/Other/Server System 1.0/pgsql9.5/bin/postgres.exe "-D" "E:\Other\data"`,
		expected + `.old "-D" "E:\data"`,
		expected + ` "-D" "..\data"`,
		expected + ` "-D" "E:\data" "-D" "E:\Other\data"`,
		expected + ` "--single" "-D" "E:\data"`,
		`postgres.exe "-D" "E:\data"`,
	} {
		if _, _, err := postgresPostmasterArguments(line, expected); err == nil {
			t.Fatalf("accepted ambiguous postmaster identity: %q", line)
		}
	}
}

func TestPostmasterOptionsUnicodePath(t *testing.T) {
	// Case folding must not change byte offsets used to extract the prefix.
	expected := `E:\İPOS\Server System 1.0\bin\postgres.exe`
	_, args, err := postgresPostmasterArguments(expected+` "-D" "E:\İPOS\data"`, expected)
	if err != nil || !reflect.DeepEqual(args, []string{"-D", `E:\İPOS\data`}) {
		t.Fatalf("Unicode installation rejected: %q, %v", args, err)
	}
}

func TestVerifyPostgresProcessStillRejectsWrongExecutable(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pid := windows.GetCurrentProcessId()
	if err := verifyPostgresProcess(pid, pid, executable); err != nil {
		t.Fatalf("read-only inspection of current process failed: %v", err)
	}
	if err := verifyPostgresProcess(pid, pid, executable+".other"); err == nil || !strings.Contains(err.Error(), "instalasi lain") {
		t.Fatalf("wrong executable accepted: %v", err)
	}
	if err := verifyPostgresProcess(pid, 0, executable); err == nil {
		t.Fatal("empty service PID accepted")
	}
}

func TestPostgresQueryPrivilegeIsolatedAndRestoredOnError(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	caller := windows.GetCurrentThreadId()
	var token windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token); !errors.Is(err, windows.ERROR_NO_TOKEN) {
		if err == nil {
			token.Close()
		}
		t.Skip("test caller already has an impersonation token")
	}
	sentinel := errors.New("query fixture failed")
	called := false
	handle, err := withPostgresQueryPrivilege(func() (windows.Handle, error) {
		called = true
		if windows.GetCurrentThreadId() == caller {
			return 0, errors.New("query ran on caller thread")
		}
		var active windows.Token
		if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &active); err != nil {
			return 0, err
		}
		active.Close()
		return 0, sentinel
	})
	if handle != 0 {
		windows.CloseHandle(handle)
		t.Fatal("failed query returned a handle")
	}
	if called {
		if !errors.Is(err, sentinel) {
			t.Fatalf("query error lost: %v", err)
		}
	} else if !errors.Is(err, windows.ERROR_NOT_ALL_ASSIGNED) && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("unexpected privilege error: %v", err)
	}
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token); !errors.Is(err, windows.ERROR_NO_TOKEN) {
		if err == nil {
			token.Close()
		}
		t.Fatalf("caller token changed: %v", err)
	}
}

// Explicitly opt in to read-only checks against an existing installation. This
// never invokes tuning, SQL, writes, or service control and is skipped in CI.
func TestSelectedPostgresMetadataReadOnly(t *testing.T) {
	bin, name := os.Getenv("NUSA_TEST_POSTGRES_BIN"), os.Getenv("NUSA_TEST_POSTGRES_SERVICE")
	if bin == "" || name == "" {
		t.Skip("set NUSA_TEST_POSTGRES_BIN and NUSA_TEST_POSTGRES_SERVICE to inspect metadata")
	}
	managerHandle, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		t.Fatal(err)
	}
	manager := &mgr.Mgr{Handle: managerHandle}
	defer manager.Disconnect()
	command, err := postgresServiceImagePath(manager, name)
	if err != nil || !isSelectedPostgresService(command, bin) {
		t.Fatalf("service does not match selected installation: %v", err)
	}
	executable, args, err := postgresServiceArguments(command)
	if err != nil {
		t.Fatal(err)
	}
	config, directory, err := readPostgresConfigPaths(executable, args)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(directory + `\postmaster.pid`)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := parsePostmasterIdentity(data, directory)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := os.ReadFile(directory + `\postmaster.opts`)
	if err != nil {
		t.Fatal(err)
	}
	activeExe, activeArgs, err := postgresPostmasterArguments(string(opts), bin+`\postgres.exe`)
	if err != nil {
		t.Fatal(err)
	}
	activeConfig, activeDirectory, err := readPostgresConfigPaths(activeExe, activeArgs)
	if err != nil || !samePostgresPath(config, activeConfig) || !samePostgresPath(directory, activeDirectory) {
		t.Fatalf("SCM/postmaster configuration identity mismatch: %v", err)
	}
	if err := validatePostgresConfiguredPort(activeExe, activeArgs, port); err != nil {
		t.Fatal(err)
	}
	t.Logf("Selected service %s: metadata/config identity valid; active port %d; no changes applied", name, port)
}
