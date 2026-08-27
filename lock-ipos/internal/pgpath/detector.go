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
}

const (
	maxNestedSearchDepth       = 3
	maxNestedSearchDirectories = 512
)

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

	matches, searchLimitReached := findNestedPostgreSQLBins(cleanPath)
	if searchLimitReached {
		return "", errors.New("folder terlalu luas untuk dipindai; pilih folder yang lebih dekat ke instalasi PostgreSQL")
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

type nestedSearchDir struct {
	path  string
	depth int
}

func findNestedPostgreSQLBins(root string) ([]string, bool) {
	queue := []nestedSearchDir{{path: root}}
	matches := make([]string, 0, 1)
	searchedDirectories := 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth > 0 {
			candidate := filepath.Join(current.path, "bin")
			if regularFileExists(filepath.Join(candidate, "psql.exe")) {
				matches = append(matches, candidate)
			}
		}
		if current.depth >= maxNestedSearchDepth {
			continue
		}

		entries, err := os.ReadDir(current.path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			searchedDirectories++
			if searchedDirectories > maxNestedSearchDirectories {
				return matches, true
			}
			queue = append(queue, nestedSearchDir{
				path:  filepath.Join(current.path, entry.Name()),
				depth: current.depth + 1,
			})
		}
	}

	return matches, false
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
