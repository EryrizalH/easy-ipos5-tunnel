package db

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Connection constants (hardcoded as per requirements)
const (
	Host     = "localhost"
	Port     = "5444"
	User     = "sysi5adm"
	Password = "u&aV23cc.o82dtr1x89c"
	Database = "postgres"
)

// GetCurrentPermission checks if the current user can create databases
func GetCurrentPermission(pgBinPath string) (bool, error) {
	query := "SELECT usecreatedb FROM pg_user WHERE usename = current_user;"
	output, err := executePSQL(pgBinPath, query)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	// Parse output - expected format: "usecreatedb\n--------------\nt or f\n(row count)"
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "t" {
			return true, nil
		} else if line == "f" {
			return false, nil
		}
	}

	return false, fmt.Errorf("could not parse permission output: %s", output)
}

// SetPermission updates the user's CREATEDB permission
func SetPermission(pgBinPath string, allowCreateDB bool) error {
	var query string
	if allowCreateDB {
		query = fmt.Sprintf("ALTER USER %s CREATEDB;", User)
	} else {
		query = fmt.Sprintf("ALTER USER %s NOCREATEDB;", User)
	}

	return executePSQLCommand(pgBinPath, query)
}

// GetSQLCommandForPreview returns the SQL command string for preview
func GetSQLCommandForPreview(allowCreateDB bool) string {
	if allowCreateDB {
		return fmt.Sprintf("ALTER USER %s CREATEDB;", User)
	}
	return fmt.Sprintf("ALTER USER %s NOCREATEDB;", User)
}

// executePSQL executes a psql command and returns the output
func executePSQL(pgBinPath, query string) (string, error) {
	psqlPath := fmt.Sprintf("%s\\psql.exe", pgBinPath)

	cmd := exec.Command(psqlPath,
		"-h", Host,
		"-p", Port,
		"-U", User,
		"-d", Database,
		"-c", query,
		"-t", // tuples-only - remove headers and footers
	)

	// Set password via environment variable
	cmd.Env = append(os.Environ(), "PGPASSWORD="+Password)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("psql error: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// executePSQLCommand executes a psql command without returning output
func executePSQLCommand(pgBinPath, query string) error {
	psqlPath := fmt.Sprintf("%s\\psql.exe", pgBinPath)

	cmd := exec.Command(psqlPath,
		"-h", Host,
		"-p", Port,
		"-U", User,
		"-d", Database,
		"-c", query,
	)

	// Set password via environment variable
	cmd.Env = append(os.Environ(), "PGPASSWORD="+Password)

	// Capture stderr for error messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("psql error: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// ExecuteAsUser executes a psql command as a specific user without password (for trust auth)
// This is used when trust authentication is temporarily enabled
func ExecuteAsUser(pgBinPath, username, database, query string) error {
	psqlPath := fmt.Sprintf("%s\\psql.exe", pgBinPath)

	cmd := exec.Command(psqlPath,
		"-h", Host,
		"-p", Port,
		"-U", username,
		"-d", database,
		"-c", query,
	)

	// No password set - relies on trust authentication
	// cmd.Env = append(os.Environ(), "PGPASSWORD="+Password)

	// Capture stderr for error messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("psql error: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// TestConnection tests if the PostgreSQL connection is working
func TestConnection(pgBinPath string) error {
	query := "SELECT 1;"
	_, err := executePSQL(pgBinPath, query)
	return err
}
