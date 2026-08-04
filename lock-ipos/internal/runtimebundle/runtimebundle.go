package runtimebundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxPayloadSize = 64 << 20

// Extract copies the ZIP payload appended to installerPath into destination.
// Only the declared runtime files are accepted to keep the installer payload bounded.
func Extract(installerPath, destination string, required, optional []string) error {
	archive, err := zip.OpenReader(installerPath)
	if err != nil {
		return fmt.Errorf("payload runtime di setup.exe tidak valid: %w", err)
	}
	defer archive.Close()

	expected := make(map[string]struct{}, len(required))
	for _, name := range required {
		expected[name] = struct{}{}
	}
	for _, name := range optional {
		expected[name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(required))

	for _, file := range archive.File {
		if _, ok := expected[file.Name]; !ok || file.FileInfo().IsDir() {
			return fmt.Errorf("payload runtime berisi file tidak dikenal: %s", file.Name)
		}
		if file.UncompressedSize64 == 0 {
			return fmt.Errorf("payload runtime kosong: %s", file.Name)
		}
		if file.UncompressedSize64 > maxPayloadSize {
			return fmt.Errorf("payload runtime terlalu besar: %s", file.Name)
		}
		if _, ok := seen[file.Name]; ok {
			return fmt.Errorf("payload runtime memuat file ganda: %s", file.Name)
		}

		reader, err := file.Open()
		if err != nil {
			return fmt.Errorf("gagal membuka payload %s: %w", file.Name, err)
		}
		path := filepath.Join(destination, file.Name)
		writer, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			_ = reader.Close()
			return fmt.Errorf("gagal membuat payload %s: %w", file.Name, err)
		}
		_, err = io.Copy(writer, reader)
		closeErr := writer.Close()
		readErr := reader.Close()
		if err != nil {
			return fmt.Errorf("gagal mengekstrak payload %s: %w", file.Name, err)
		}
		if closeErr != nil {
			return fmt.Errorf("gagal menyimpan payload %s: %w", file.Name, closeErr)
		}
		if readErr != nil {
			return fmt.Errorf("gagal menutup payload %s: %w", file.Name, readErr)
		}
		seen[file.Name] = struct{}{}
	}

	for _, name := range required {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("payload runtime tidak lengkap: %s", name)
		}
	}
	return nil
}
