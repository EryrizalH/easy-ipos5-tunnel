package main

import (
	"testing"

	"github.com/lock-ipos/lock-ipos/internal/progress"
)

func TestNewProgressStateData_InitializesPendingSteps(t *testing.T) {
	title, steps := progressPlan(optionInstallPgBouncer)
	data := newProgressStateData(title, steps)

	if data.Title != "Install PgBouncer" {
		t.Fatalf("unexpected title: %s", data.Title)
	}
	if len(data.Steps) != len(steps) {
		t.Fatalf("expected %d steps, got %d", len(steps), len(data.Steps))
	}
	for _, step := range data.Steps {
		if step.Status != progress.StepPending {
			t.Fatalf("expected pending status for %s, got %s", step.ID, step.Status)
		}
	}
}

func TestProgressStateData_AppliesStepEventsAndLogTail(t *testing.T) {
	_, steps := progressPlan(optionLockDB)
	data := newProgressStateData("Kunci Pembuatan Database", steps)

	data.startStep(progress.StepStartedMsg{ID: "find-pghba", Label: "Mencari pg_hba.conf"})
	data.finishStep(progress.StepFinishedMsg{ID: "find-pghba", Success: true, Detail: "Lokasi ditemukan"})
	data.startStep(progress.StepStartedMsg{ID: "backup-pghba", Label: "Membuat backup pg_hba.conf"})
	data.Logs.Append("log-1")
	data.Logs.Append("log-2")

	if data.Steps[0].Status != progress.StepSuccess {
		t.Fatalf("expected first step success, got %s", data.Steps[0].Status)
	}
	if data.Steps[0].Detail != "Lokasi ditemukan" {
		t.Fatalf("unexpected first step detail: %s", data.Steps[0].Detail)
	}
	if data.Steps[1].Status != progress.StepRunning {
		t.Fatalf("expected second step running, got %s", data.Steps[1].Status)
	}

	logs := data.Logs.Lines()
	if len(logs) != 2 || logs[0] != "log-1" || logs[1] != "log-2" {
		t.Fatalf("unexpected logs: %#v", logs)
	}
}
