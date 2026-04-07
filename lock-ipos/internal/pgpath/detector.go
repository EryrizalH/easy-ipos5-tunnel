package pgpath

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Default PostgreSQL bin paths to check
var defaultPaths = []string{
	`C:\Program Files (x86)\Inspirasibiz\Server System 1.0\pgsql9.5\bin`,
	`D:\Server System 1.0\pgsql9.5\bin`,
}

// FindPostgreSQLBin searches for PostgreSQL binary in default locations
func FindPostgreSQLBin() (string, error) {
	for _, path := range defaultPaths {
		if fileExists(filepath.Join(path, "psql.exe")) {
			return path, nil
		}
	}
	return "", errors.New("PostgreSQL bin tidak ditemukan di lokasi default")
}

// FindPostgreSQLBinInPath searches for psql.exe in a specific path
func FindPostgreSQLBinInPath(customPath string) (string, error) {
	// Clean and validate the path
	cleanPath := strings.TrimSpace(customPath)
	if cleanPath == "" {
		return "", errors.New("Path tidak boleh kosong")
	}

	// Check if psql.exe exists in the provided path
	psqlPath := filepath.Join(cleanPath, "psql.exe")
	if !fileExists(psqlPath) {
		// Also check if the path already includes psql.exe
		if strings.HasSuffix(strings.ToLower(cleanPath), "psql.exe") {
			if fileExists(cleanPath) {
				return filepath.Dir(cleanPath), nil
			}
		}
		return "", errors.New("psql.exe tidak ditemukan di path yang diberikan")
	}

	return cleanPath, nil
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	info, err := filepath.Abs(filename)
	if err != nil {
		return false
	}
	_, err = os.Stat(info)
	return err == nil
}

// GetDefaultPaths returns the list of default PostgreSQL bin paths
func GetDefaultPaths() []string {
	return defaultPaths
}

// FindPostgreSQLData searches for PostgreSQL data directory
func FindPostgreSQLData(binPath string) (string, error) {
	// The data directory is typically a sibling of the bin directory
	binDir := binPath
	if strings.HasSuffix(strings.ToLower(binDir), "bin") {
		binDir = filepath.Dir(binPath)
	}

	parentDir := filepath.Dir(binDir)
	dataDir := filepath.Join(parentDir, "data")

	// Check if data directory exists
	if _, err := os.Stat(dataDir); err != nil {
		return "", err
	}

	return dataDir, nil
}
