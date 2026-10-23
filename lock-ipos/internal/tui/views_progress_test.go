package tui

import (
	"strings"
	"testing"
)

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
