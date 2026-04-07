package pgadmin

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lock-ipos/lock-ipos/internal/logger"
)

// FindPgHbaConf locates the pg_hba.conf file from the PostgreSQL bin path
func FindPgHbaConf(pgBinPath string) (string, error) {
	logger.Info("Searching for pg_hba.conf...")

	// pg_hba.conf is typically in the data directory, which is a sibling of bin
	// Bin path pattern: .../pgsql9.5/bin
	// Data path pattern: .../pgsql9.5/data
	binDir := filepath.Dir(pgBinPath)
	parentDir := filepath.Dir(binDir)
	dataDir := filepath.Join(parentDir, "data")

	logger.Debugf("Bin dir: %s", binDir)
	logger.Debugf("Parent dir: %s", parentDir)
	logger.Debugf("Data dir: %s", dataDir)

	// List of possible paths to check, in order of preference
	possiblePaths := []string{
		filepath.Join(dataDir, "pg_hba.conf"),
		filepath.Join(parentDir, "data", "pg_hba.conf"),
		filepath.Join(binDir, "..", "data", "pg_hba.conf"),
		filepath.Join(binDir, "..", "..", "data", "pg_hba.conf"),
		// Also try directly in parentDir
		filepath.Join(parentDir, "pg_hba.conf"),
		// Try with pgsql9.5 as the base
		filepath.Join(parentDir, "..", "pgsql9.5", "data", "pg_hba.conf"),
		// Try D: drive variant
		`D:\Server System 1.0\pgsql9.5\data\pg_hba.conf`,
		`D:\Server System 1.0\pgsql9.5\data\pg_hba.conf`,
	}

	for i, pgHbaPath := range possiblePaths {
		// Convert to absolute path for cleaner output
		absPath, err := filepath.Abs(pgHbaPath)
		if err != nil {
			absPath = pgHbaPath
		}

		logger.Debugf("Checking path %d: %s", i+1, absPath)

		if _, err := os.Stat(absPath); err == nil {
			logger.Infof("Found pg_hba.conf at: %s", absPath)
			return absPath, nil
		} else {
			logger.Debugf("Path %d not accessible: %v", i+1, err)
		}
	}

	// Fallback: Try direct search in common paths
	logger.Info("Path-based search failed, trying direct search in common locations...")
	return FindPgHbaConfDirectly()
}

// BackupPgHbaConf creates a timestamped backup of pg_hba.conf
func BackupPgHbaConf(pgHbaPath string) (string, error) {
	timestamp := time.Now().Format("20060102150405")
	backupPath := pgHbaPath + ".backup." + timestamp

	logger.Infof("Creating backup of pg_hba.conf to: %s", backupPath)

	// Read original file
	content, err := os.ReadFile(pgHbaPath)
	if err != nil {
		return "", NewPgHbaError("backup", fmt.Errorf("failed to read pg_hba.conf: %w", err))
	}

	// Write backup
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", NewPgHbaError("backup", fmt.Errorf("failed to write backup: %w", err))
	}

	logger.Infof("Backup created successfully at: %s", backupPath)
	return backupPath, nil
}

// FindPgHbaConfDirectly searches for pg_hba.conf by trying common installation paths
// This is a fallback method that scans typical PostgreSQL data locations
func FindPgHbaConfDirectly() (string, error) {
	logger.Info("Performing direct search for pg_hba.conf in common paths...")

	// Common PostgreSQL data directory locations on Windows
	commonPaths := []string{
		`C:\Program Files (x86)\Inspirasibiz\Server System 1.0\pgsql9.5\data\pg_hba.conf`,
		`C:\Program Files (x86)\Inspirasibiz\Server System 1.0\pgsql9.5\data\pg_hba.conf`,
		`D:\Server System 1.0\pgsql9.5\data\pg_hba.conf`,
		`C:\Program Files (x86)\PostgreSQL\9.5\data\pg_hba.conf`,
		`C:\Program Files\PostgreSQL\9.5\data\pg_hba.conf`,
		`D:\PostgreSQL\9.5\data\pg_hba.conf`,
		`C:\PostgreSQL\9.5\data\pg_hba.conf`,
		// Also check parent directories
		`C:\Program Files (x86)\Inspirasibiz\data\pg_hba.conf`,
		`C:\Program Files (x86)\Inspirasibiz\pgsql9.5\data\pg_hba.conf`,
	}

	for _, path := range commonPaths {
		logger.Debugf("Checking: %s", path)
		if _, err := os.Stat(path); err == nil {
			logger.Infof("Found pg_hba.conf at: %s", path)
			return path, nil
		}
	}

	return "", NewPgHbaError("direct search", fmt.Errorf("pg_hba.conf not found in any common Windows PostgreSQL paths"))
}

// SetAuthMethodToTrust modifies pg_hba.conf to use 'trust' instead of 'md5' for local connections
func SetAuthMethodToTrust(pgHbaPath string) error {
	logger.Info("Modifying pg_hba.conf to use 'trust' authentication...")

	// Read the file
	file, err := os.Open(pgHbaPath)
	if err != nil {
		return NewPgHbaError("open file", err)
	}
	defer file.Close()

	var lines []string
	changesCount := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is an auth method line with md5
		// Common patterns:
		// local   all             all                                     md5
		// host    all             all             127.0.0.1/32            md5
		// host    all             all             ::1/128                 md5
		if strings.HasSuffix(strings.TrimSpace(line), "md5") {
			// Replace md5 with trust
			modifiedLine := strings.TrimSuffix(strings.TrimSpace(line), "md5") + "trust"
			lines = append(lines, modifiedLine)
			changesCount++
			logger.Debugf("Changed line: %s -> %s", line, modifiedLine)
		} else {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return NewPgHbaError("read file", err)
	}
	file.Close()

	// Write the modified content back
	if err := os.WriteFile(pgHbaPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return NewPgHbaError("write file", err)
	}

	logger.Infof("Modified pg_hba.conf: changed %d lines from md5 to trust", changesCount)
	return nil
}

// RestorePgHbaConf restores the original pg_hba.conf from backup
func RestorePgHbaConf(originalPath, backupPath string) error {
	logger.Infof("Restoring pg_hba.conf from backup: %s", backupPath)

	// Read backup content
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return NewPgHbaError("restore", fmt.Errorf("failed to read backup file: %w", err))
	}

	// Write to original location
	if err := os.WriteFile(originalPath, content, 0644); err != nil {
		return NewPgHbaError("restore", fmt.Errorf("failed to restore pg_hba.conf: %w", err))
	}

	logger.Info("pg_hba.conf restored successfully")
	return nil
}
