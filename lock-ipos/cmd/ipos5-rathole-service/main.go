package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/lock-ipos/lock-ipos/internal/servicewrapper"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal membaca path executable:", err)
		os.Exit(1)
	}

	cfg, err := servicewrapper.ParseArgs(os.Args[1:], filepath.Dir(exePath))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal parse argumen:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := servicewrapper.Run(ctx, cfg, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
