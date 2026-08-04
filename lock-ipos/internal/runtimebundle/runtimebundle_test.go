package runtimebundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeInstallerPayload(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("MZstub")); err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(f)
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractSelfExtractingPayload(t *testing.T) {
	tmp := t.TempDir()
	installer := filepath.Join(tmp, "setup.exe")
	writeInstallerPayload(t, installer, map[string]string{"nssm.exe": "nssm", "nusatunnel.exe": "tunnel"})

	destination := filepath.Join(tmp, "runtime")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Extract(installer, destination, []string{"nssm.exe", "nusatunnel.exe"}, nil); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(destination, "nusatunnel.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tunnel" {
		t.Fatalf("unexpected payload: %q", data)
	}
}

func TestExtractRejectsIncompletePayload(t *testing.T) {
	tmp := t.TempDir()
	installer := filepath.Join(tmp, "setup.exe")
	writeInstallerPayload(t, installer, map[string]string{"nssm.exe": "nssm"})

	destination := filepath.Join(tmp, "runtime")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Extract(installer, destination, []string{"nssm.exe", "nusatunnel.exe"}, nil); err == nil {
		t.Fatal("expected incomplete payload error")
	}
}
