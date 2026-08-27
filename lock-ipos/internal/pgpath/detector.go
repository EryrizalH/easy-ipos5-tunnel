package pgpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Default PostgreSQL bin paths to check
var defaultPaths = []string{
	`C:\Program Files (x86)\Inspirasibiz\Server System 1.0\pgsql9.5\bin`,
	`D:\Server System 1.0\pgsql9.5\bin`,
	`D:\IPOS 5 data\Server System 1.0\pgsql9.5\bin`,
}

// FindPostgreSQLBin searches for PostgreSQL binary in default locations
func FindPostgreSQLBin() (string, error) {
	for _, path := range defaultPaths {
		if regularFileExists(filepath.Join(path, "psql.exe")) {
			return path, nil
		}
	}
	return "", errors.New("PostgreSQL bin tidak ditemukan di lokasi default")
}

// FindPostgreSQLBinInPath searches for psql.exe in a specific path
func FindPostgreSQLBinInPath(customPath string) (string, error) {
	cleanPath, err := normalizeInputPath(customPath)
	if err != nil {
		return "", err
	}

	info, statErr := os.Stat(cleanPath)
	if statErr != nil {
		return "", fmt.Errorf("path tidak ditemukan atau tidak dapat dibaca: %s (%w)", cleanPath, statErr)
	}

	if !info.IsDir() {
		if strings.EqualFold(filepath.Base(cleanPath), "psql.exe") && info.Mode().IsRegular() {
			return filepath.Dir(cleanPath), nil
		}
		return "", errors.New("file yang dipilih bukan psql.exe")
	}

	if regularFileExists(filepath.Join(cleanPath, "psql.exe")) {
		return cleanPath, nil
	}

	binPath := filepath.Join(cleanPath, "bin")
	if regularFileExists(filepath.Join(binPath, "psql.exe")) {
		return binPath, nil
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return "", fmt.Errorf("folder tidak dapat dibaca: %w", err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(cleanPath, entry.Name(), "bin")
		if regularFileExists(filepath.Join(candidate, "psql.exe")) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf(
			"psql.exe tidak ditemukan; sudah dicek %s dan %s",
			filepath.Join(cleanPath, "psql.exe"),
			filepath.Join(cleanPath, "bin", "psql.exe"),
		)
	default:
		return "", errors.New("lebih dari satu instalasi PostgreSQL ditemukan; pilih folder bin yang digunakan IPOS 5")
	}
}

func normalizeInputPath(input string) (string, error) {
	cleanPath := strings.TrimSpace(input)
	if cleanPath == "" {
		return "", errors.New("path tidak boleh kosong")
	}

	for len(cleanPath) >= 2 {
		first := cleanPath[0]
		last := cleanPath[len(cleanPath)-1]
		if (first != '"' || last != '"') && (first != '\'' || last != '\'') {
			break
		}
		cleanPath = strings.TrimSpace(cleanPath[1 : len(cleanPath)-1])
	}
	if cleanPath == "" {
		return "", errors.New("path tidak boleh kosong")
	}

	absPath, err := filepath.Abs(filepath.Clean(cleanPath))
	if err != nil {
		return "", fmt.Errorf("path tidak valid: %w", err)
	}
	return filepath.Clean(absPath), nil
}

func regularFileExists(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && info.Mode().IsRegular()
}

// GetDefaultPaths returns the list of default PostgreSQL bin paths
func GetDefaultPaths() []string {
	return defaultPaths
}

// FindPostgreSQLData searches for PostgreSQL data directory
func FindPostgreSQLData(binPath string) (string, error) {
	// The data directory is typically a sibling of the bin directory
	postgresDir := filepath.Clean(binPath)
	if strings.EqualFold(filepath.Base(postgresDir), "bin") {
		postgresDir = filepath.Dir(postgresDir)
	}

	dataDir := filepath.Join(postgresDir, "data")

	// Check if data directory exists
	if _, err := os.Stat(dataDir); err != nil {
		return "", err
	}

	return dataDir, nil
}
