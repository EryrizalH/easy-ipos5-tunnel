//go:build windows

package winservice

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openPostgresProcessForQuery(pid uint32) (windows.Handle, error) {
	query := func() (windows.Handle, error) {
		return windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	}
	process, err := query()
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return process, err
	}
	// PostgreSQL's restricted service token can deny image inspection even to
	// administrators. Retry only this read-only open with an existing admin
	// privilege; do not alter the process DACL or grant any account new rights.
	process, err = withPostgresQueryPrivilege(query)
	if err != nil {
		return 0, fmt.Errorf("izin pemeriksaan proses PostgreSQL tidak tersedia; jalankan setup sebagai Administrator dengan SeDebugPrivilege: %w", err)
	}
	return process, nil
}

// Isolate impersonation on a dedicated locked worker. Restore its token before
// returning a handle. If restoration fails, retire that OS thread rather than
// returning it to Go's thread pool with an impersonation token still attached.
func withPostgresQueryPrivilege(query func() (windows.Handle, error)) (windows.Handle, error) {
	type result struct {
		handle windows.Handle
		err    error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		unlock := true
		defer func() {
			if unlock {
				runtime.UnlockOSThread()
			}
		}()
		handle, err := func() (handle windows.Handle, resultErr error) {
			var existing windows.Token
			err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &existing)
			if err == nil {
				existing.Close()
				return 0, errors.New("thread pemeriksaan sudah menggunakan token impersonasi")
			}
			if !errors.Is(err, windows.ERROR_NO_TOKEN) {
				return 0, err
			}
			if err := windows.ImpersonateSelf(windows.SecurityImpersonation); err != nil {
				return 0, err
			}
			defer func() {
				if err := windows.RevertToSelf(); err != nil {
					unlock = false
					if handle != 0 {
						windows.CloseHandle(handle)
						handle = 0
					}
					resultErr = fmt.Errorf("gagal memulihkan token thread pemeriksaan: %w", err)
				}
			}()
			var token windows.Token
			if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, true, &token); err != nil {
				return 0, err
			}
			defer token.Close()
			var privilege windows.LUID
			if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeDebugPrivilege"), &privilege); err != nil {
				return 0, err
			}
			state := windows.Tokenprivileges{PrivilegeCount: 1}
			state.Privileges[0] = windows.LUIDAndAttributes{Luid: privilege, Attributes: windows.SE_PRIVILEGE_ENABLED}
			// x/sys v0.18 ignores ERROR_NOT_ALL_ASSIGNED on a nonzero return.
			// Windows explicitly sets last-error on success as well as failure.
			adjust := windows.NewLazySystemDLL("advapi32.dll").NewProc("AdjustTokenPrivileges")
			ok, _, callErr := adjust.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&state)), 0, 0, 0)
			if ok == 0 || callErr != windows.ERROR_SUCCESS {
				return 0, fmt.Errorf("SeDebugPrivilege tidak dapat diaktifkan: %w", callErr)
			}
			return query()
		}()
		done <- result{handle, err}
	}()
	value := <-done
	return value.handle, value.err
}
