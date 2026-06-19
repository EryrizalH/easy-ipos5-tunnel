package appcore

import "testing"

func TestBuildTaskSchedulerRunCommand_QuotesExecutablePathWithSpaces(t *testing.T) {
	got := buildTaskSchedulerRunCommand(`C:\Program Files\NusaTunnel\nusatunnel-gui.exe`)
	want := `"C:\Program Files\NusaTunnel\nusatunnel-gui.exe" --hidden`
	if got != want {
		t.Fatalf("unexpected task command:\nwant: %s\n got: %s", want, got)
	}
}
