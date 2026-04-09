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
