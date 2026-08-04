package winservice

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lock-ipos/lock-ipos/internal/progress"
)

type captureReporter struct {
	steps []string
	logs  []string
}

func (r *captureReporter) StartStep(id, label string) {
	r.steps = append(r.steps, "start:"+id+":"+label)
}

func (r *captureReporter) FinishStep(id string, success bool, detail string) {
	status := "fail"
	if success {
		status = "success"
	}
	r.steps = append(r.steps, status+":"+id+":"+detail)
}

func (r *captureReporter) Log(line string) {
	r.logs = append(r.logs, line)
}

func (r *captureReporter) Summary(summary string) {}

func TestRunWithReporter_StreamsCommandLogs(t *testing.T) {
	reporter := &captureReporter{}

	command, args := "cmd", []string{"/c", "echo hello && echo world 1>&2"}
	if runtime.GOOS != "windows" {
		command, args = "sh", []string{"-c", "echo hello; echo world >&2"}
	}
	out, err := runWithReporter(reporter, command, args...)
	if err != nil {
		t.Fatalf("runWithReporter() error = %v", err)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Fatalf("expected combined output to include stdout and stderr, got %q", out)
	}

	joinedLogs := strings.Join(reporter.logs, "\n")
	checks := []string{
		"Menjalankan: " + command + " " + strings.Join(args, " "),
		"hello",
		"world",
	}
	for _, needle := range checks {
		if !strings.Contains(joinedLogs, needle) {
			t.Fatalf("expected logs to contain %q, got %q", needle, joinedLogs)
		}
	}
}

func TestWaitServiceStateWithQuery_StillReusableWithProgressWrapper(t *testing.T) {
	reporter := progress.NopReporter()
	_ = reporter
	calls := 0
	err := waitServiceStateWithQuery("PgBouncer", "RUNNING", 50*time.Millisecond, 0,
		func(_ string) (string, string, error) {
			calls++
			if calls < 2 {
				return "START_PENDING", "STATE : 2 START_PENDING", nil
			}
			return "RUNNING", "STATE : 4 RUNNING", nil
		},
		func(time.Duration) {},
	)
	if err != nil {
		t.Fatalf("waitServiceStateWithQuery() error = %v", err)
	}
}
