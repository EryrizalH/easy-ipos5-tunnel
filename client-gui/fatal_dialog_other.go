//go:build !windows

package main

import (
	"fmt"
	"os"
)

func showFatalDialog(title string, message string) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %s\n", title, message)
}
