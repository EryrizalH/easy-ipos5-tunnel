package main

import (
	"context"
	"testing"
)

func TestBeforeCloseHidesWindowWhenNotQuitting(t *testing.T) {
	app := &App{}
	hidden := 0
	app.hideWindow = func(context.Context) {
		hidden++
	}

	prevent := app.beforeClose(context.Background())

	if !prevent {
		t.Fatalf("expected close to be prevented when not quitting")
	}
	if hidden != 1 {
		t.Fatalf("expected hideWindow to be called once, got %d", hidden)
	}
}

func TestBeforeCloseAllowsQuitWhenQuitting(t *testing.T) {
	app := &App{}
	hidden := 0
	app.hideWindow = func(context.Context) {
		hidden++
	}
	app.setQuitting()

	prevent := app.beforeClose(context.Background())

	if prevent {
		t.Fatalf("expected close not to be prevented while quitting")
	}
	if hidden != 0 {
		t.Fatalf("expected hideWindow not to be called while quitting, got %d", hidden)
	}
}

func TestQuitTrayIsIdempotent(t *testing.T) {
	app := &App{}
	called := 0
	app.quitTrayFn = func() {
		called++
	}

	app.quitTray()
	app.quitTray()

	if called != 1 {
		t.Fatalf("expected tray quit to run once, got %d", called)
	}
}
