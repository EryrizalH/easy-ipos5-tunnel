package progress

import (
	"strings"
	"sync"
)

type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepSuccess StepStatus = "success"
	StepFailed  StepStatus = "failed"
)

type StepDefinition struct {
	ID    string
	Label string
}

type StepStartedMsg struct {
	ID     string
	Label  string
	Detail string
}

type StepFinishedMsg struct {
	ID      string
	Label   string
	Success bool
	Detail  string
}

type LogLineMsg struct {
	Line string
}

type SummaryUpdatedMsg struct {
	Summary string
}

type Reporter interface {
	StartStep(id, label string)
	FinishStep(id string, success bool, detail string)
	Log(line string)
	Summary(summary string)
}

type nopReporter struct{}

func (nopReporter) StartStep(string, string)        {}
func (nopReporter) FinishStep(string, bool, string) {}
func (nopReporter) Log(string)                      {}
func (nopReporter) Summary(string)                  {}

func NopReporter() Reporter {
	return nopReporter{}
}

type channelReporter struct {
	ch chan<- any
}

func NewChannelReporter(ch chan<- any) Reporter {
	if ch == nil {
		return NopReporter()
	}
	return channelReporter{ch: ch}
}

func (r channelReporter) StartStep(id, label string) {
	r.ch <- StepStartedMsg{ID: id, Label: label}
}

func (r channelReporter) FinishStep(id string, success bool, detail string) {
	r.ch <- StepFinishedMsg{ID: id, Success: success, Detail: detail}
}

func (r channelReporter) Log(line string) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
	if trimmed == "" {
		return
	}
	r.ch <- LogLineMsg{Line: trimmed}
}

func (r channelReporter) Summary(summary string) {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return
	}
	r.ch <- SummaryUpdatedMsg{Summary: trimmed}
}

type LineBuffer struct {
	limit int
	mu    sync.Mutex
	lines []string
}

func NewLineBuffer(limit int) *LineBuffer {
	if limit <= 0 {
		limit = 10
	}
	return &LineBuffer{limit: limit, lines: make([]string, 0, limit)}
}

func (b *LineBuffer) Append(line string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, part := range strings.Split(strings.ReplaceAll(line, "\r", ""), "\n") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		b.lines = append(b.lines, trimmed)
		if len(b.lines) > b.limit {
			b.lines = append([]string(nil), b.lines[len(b.lines)-b.limit:]...)
		}
	}
}

func (b *LineBuffer) Lines() []string {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.lines...)
}
