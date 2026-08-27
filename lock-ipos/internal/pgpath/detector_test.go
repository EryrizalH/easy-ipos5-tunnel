package pgpath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPostgreSQLBinInPathAcceptsCommonPathLevels(t *testing.T) {
	serverRoot := filepath.Join(t.TempDir(), "IPOS 5 data", "Server System 1.0")
	binPath := filepath.Join(serverRoot, "pgsql9.5", "bin")
	psqlPath := filepath.Join(binPath, "psql.exe")
	writeTestFile(t, psqlPath)

	tests := []struct {
		name  string
		input string
	}{
		{name: "installation parent", input: filepath.Dir(serverRoot)},
		{name: "server root", input: serverRoot},
		{name: "postgres root", input: filepath.Dir(binPath)},
		{name: "bin folder", input: binPath},
		{name: "psql file", input: psqlPath},
		{name: "quoted psql file", input: `"` + psqlPath + `"`},
		{name: "single quoted bin", input: `'` + binPath + `'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindPostgreSQLBinInPath(tt.input)
			if err != nil {
				t.Fatalf("FindPostgreSQLBinInPath(%q) error: %v", tt.input, err)
			}
			if got != binPath {
				t.Fatalf("expected bin path %q, got %q", binPath, got)
			}
		})
	}
}

func TestFindPostgreSQLBinInPathRejectsAmbiguousRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "pgsql9.5", "bin", "psql.exe"))
	writeTestFile(t, filepath.Join(root, "pgsql14", "bin", "psql.exe"))

	_, err := FindPostgreSQLBinInPath(root)
	if err == nil || !strings.Contains(err.Error(), "lebih dari satu") {
		t.Fatalf("expected ambiguous installation error, got %v", err)
	}
}

func TestFindPostgreSQLBinInPathRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: "   "},
		{name: "missing", input: filepath.Join(t.TempDir(), "missing")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FindPostgreSQLBinInPath(tt.input); err == nil {
				t.Fatalf("expected error for input %q", tt.input)
			}
		})
	}
}

func TestFindPostgreSQLBinInPathRejectsDirectoryNamedPsql(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(filepath.Join(binPath, "psql.exe"), 0o755); err != nil {
		t.Fatalf("create psql.exe directory: %v", err)
	}

	if _, err := FindPostgreSQLBinInPath(binPath); err == nil {
		t.Fatal("expected directory named psql.exe to be rejected")
	}
}

func TestFindPostgreSQLDataUsesSiblingOfBin(t *testing.T) {
	postgresDir := filepath.Join(t.TempDir(), "Server System 1.0", "pgsql9.5")
	binPath := filepath.Join(postgresDir, "bin")
	dataPath := filepath.Join(postgresDir, "data")
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("create bin directory: %v", err)
	}
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		t.Fatalf("create data directory: %v", err)
	}

	got, err := FindPostgreSQLData(binPath)
	if err != nil {
		t.Fatalf("FindPostgreSQLData() error: %v", err)
	}
	if got != dataPath {
		t.Fatalf("expected data path %q, got %q", dataPath, got)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("create test file: %v", err)
	}
}
