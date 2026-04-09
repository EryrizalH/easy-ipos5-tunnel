package progress

import "testing"

func TestLineBuffer_AppendKeepsLatestLines(t *testing.T) {
	buf := NewLineBuffer(3)
	buf.Append("satu")
	buf.Append("dua\ntiga")
	buf.Append("empat")

	got := buf.Lines()
	want := []string{"dua", "tiga", "empat"}
	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestChannelReporter_EmitsStructuredMessages(t *testing.T) {
	ch := make(chan any, 4)
	reporter := NewChannelReporter(ch)

	reporter.StartStep("validate-admin", "Validasi hak Administrator")
	reporter.Log("Menjalankan: sc query EasyRatholeClient")
	reporter.FinishStep("validate-admin", true, "Hak Administrator terdeteksi")
	reporter.Summary("Ringkasan selesai")

	msg1 := (<-ch).(StepStartedMsg)
	if msg1.ID != "validate-admin" || msg1.Label != "Validasi hak Administrator" {
		t.Fatalf("unexpected step start message: %#v", msg1)
	}

	msg2 := (<-ch).(LogLineMsg)
	if msg2.Line != "Menjalankan: sc query EasyRatholeClient" {
		t.Fatalf("unexpected log message: %#v", msg2)
	}

	msg3 := (<-ch).(StepFinishedMsg)
	if !msg3.Success || msg3.Detail != "Hak Administrator terdeteksi" {
		t.Fatalf("unexpected step finished message: %#v", msg3)
	}

	msg4 := (<-ch).(SummaryUpdatedMsg)
	if msg4.Summary != "Ringkasan selesai" {
		t.Fatalf("unexpected summary message: %#v", msg4)
	}
}
