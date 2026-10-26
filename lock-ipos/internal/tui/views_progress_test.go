package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderResultWrapsFullCauseAndLogLocation(t *testing.T) {
	message := "tuning PostgreSQL gagal; pemasangan tunnel dibatalkan: identitas PostgreSQL iPgSql.9.5 tidak pasti: proses postmaster tidak dapat diperiksa: Access is denied."
	for _, width := range []int{40, 80, 110} {
		view := RenderResult(DefaultStyles(), false, false, 1, "", message, width, `C:\setup\log.txt`)
		for _, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("line width %d exceeds terminal width %d: %s", got, width, line)
			}
		}
		// Wrapping may split words; remove whitespace and the box's vertical
		// borders so every original character must still be present in order.
		compact := func(value string) string {
			return strings.Join(strings.Fields(strings.ReplaceAll(value, "│", "")), "")
		}
		for _, want := range []string{message, `Log setup: C:\setup\log.txt`} {
			if !strings.Contains(compact(view), compact(want)) {
				t.Fatalf("missing full text %q at width %d:\n%s", want, width, view)
			}
		}
	}
}

func TestRenderResultRetainsLongUnicodeCauseWithoutLog(t *testing.T) {
	message := strings.Repeat("kesalahan 日本語 ", 40) + "PENYEBAB_AKHIR"
	view := RenderResult(DefaultStyles(), false, false, 1, "", message, 0, "")
	if !utf8.ValidString(view) || !strings.Contains(view, "PENYEBAB_AKHIR") {
		t.Fatal("long error was truncated or corrupted")
	}
	if !strings.Contains(view, "Log setup tidak tersedia") {
		t.Fatal("missing unavailable log explanation")
	}
}

func TestRenderProgress_ShowsStepsAndLogs(t *testing.T) {
	styles := DefaultStyles()
	view := RenderProgress(styles, ProgressViewModel{
		Title:   "Install PgBouncer",
		Spinner: "⠙",
		Elapsed: "3s",
		Summary: "Sedang menyiapkan runtime",
		Steps: []ProgressStepView{
			{Label: "Validasi hak Administrator", Status: "success", Detail: "Hak Administrator terdeteksi"},
			{Label: "Menyiapkan file runtime PgBouncer", Status: "running", Detail: "Membuat pgbouncer.ini"},
			{Label: "Health check PgBouncer", Status: "pending"},
		},
		Logs: []string{"Menjalankan: sc query PgBouncer", "Status service PgBouncer: RUNNING"},
	})

	checks := []string{
		"Install PgBouncer",
		"Validasi hak Administrator",
		"Menyiapkan file runtime PgBouncer",
		"Health check PgBouncer",
		"Menjalankan: sc query PgBouncer",
		"Status service PgBouncer: RUNNING",
		"3s",
	}
	for _, needle := range checks {
		if !strings.Contains(view, needle) {
			t.Fatalf("expected view to contain %q, got:\n%s", needle, view)
		}
	}
}

func TestRenderConfirm_InstallIPPublicExplainsTuningAndRestart(t *testing.T) {
	view := RenderConfirm(DefaultStyles(), 1, false, "NusaTunnelClient", `C:\runtime`)
	for _, needle := range []string{
		"max_connections=200", "work_mem=8MB", "idle=60", "interval=10", "count=0",
		"shared_buffers=25% RAM", "min 128 MB", "maks 512 MB", "Logging",
		"dibackup", "dipulihkan", "koneksi database terputus sementara",
		"Restart dilewati jika semua nilai efektif sudah sesuai",
	} {
		if !strings.Contains(view, needle) {
			t.Errorf("confirmation does not explain %q", needle)
		}
	}
}

func TestRenderConfirm_PgBouncerDoesNotOfferTuning(t *testing.T) {
	view := RenderConfirm(DefaultStyles(), 2, false, "NusaTunnelClient", `C:\runtime`)
	if strings.Contains(view, "shared_buffers") || strings.Contains(view, "max_connections=200") {
		t.Fatal("PgBouncer confirmation must not advertise PostgreSQL tuning")
	}
	if !strings.Contains(view, "127.0.0.1:5445") || !strings.Contains(view, "0.0.0.0:5444") {
		t.Fatal("PgBouncer confirmation must retain the existing port migration")
	}
}

func TestRenderConfirm_UninstallRetainsTuning(t *testing.T) {
	view := RenderConfirm(DefaultStyles(), 3, false, "NusaTunnelClient", `C:\runtime`)
	if !strings.Contains(view, "Tuning PostgreSQL tetap tersimpan") {
		t.Fatal("uninstall confirmation must explain that tuning is retained")
	}
}
