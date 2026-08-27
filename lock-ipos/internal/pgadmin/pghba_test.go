package pgadmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPgHbaConfUsesDataSiblingOfBin(t *testing.T) {
	postgresDir := filepath.Join(t.TempDir(), "IPOS 5 data", "Server System 1.0", "pgsql9.5")
	binPath := filepath.Join(postgresDir, "bin")
	pgHbaPath := filepath.Join(postgresDir, "data", "pg_hba.conf")
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("create bin directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(pgHbaPath), 0o755); err != nil {
		t.Fatalf("create data directory: %v", err)
	}
	if err := os.WriteFile(pgHbaPath, []byte("host all all 127.0.0.1/32 md5\n"), 0o644); err != nil {
		t.Fatalf("create pg_hba.conf: %v", err)
	}

	got, err := FindPgHbaConf(binPath)
	if err != nil {
		t.Fatalf("FindPgHbaConf() error: %v", err)
	}
	if got != pgHbaPath {
		t.Fatalf("expected pg_hba.conf %q, got %q", pgHbaPath, got)
	}
}
