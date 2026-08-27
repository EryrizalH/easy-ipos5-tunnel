//go:build windows

package clipboard

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cfUnicodeText = 13

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
)

// ReadText returns Unicode text currently stored in the Windows clipboard.
func ReadText() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var lastErr error
	opened := false
	for attempt := 0; attempt < 10; attempt++ {
		result, _, callErr := procOpenClipboard.Call(0)
		if result != 0 {
			opened = true
			break
		}
		lastErr = callErr
		time.Sleep(10 * time.Millisecond)
	}
	if !opened {
		return "", windowsCallError("membuka clipboard", lastErr)
	}
	defer procCloseClipboard.Call()

	handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", windowsCallError("membaca teks clipboard", callErr)
	}

	pointer, _, callErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return "", windowsCallError("mengunci data clipboard", callErr)
	}
	defer procGlobalUnlock.Call(handle)

	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(pointer))), nil
}

func windowsCallError(operation string, err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return fmt.Errorf("gagal %s", operation)
	}
	if err == nil {
		return fmt.Errorf("gagal %s", operation)
	}
	return fmt.Errorf("gagal %s: %w", operation, err)
}
