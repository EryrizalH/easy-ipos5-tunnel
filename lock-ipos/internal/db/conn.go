package db

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Connection constants (hardcoded as per requirements)
const (
	Host     = "localhost"
	Port     = "5444"
	PortNew  = "5445"
	User     = "sysi5adm"
	Password = "u&aV23cc.o82dtr1x89c"
	Database = "postgres"
)

var preferredPorts = []string{PortNew, Port}

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
	port, err := resolveReachablePort(pgBinPath)
	if err != nil {
		return "", err
	}
	return executePSQLOnPort(pgBinPath, port, User, Database, query, true)
}

// executePSQLCommand executes a psql command without returning output
func executePSQLCommand(pgBinPath, query string) error {
	port, err := resolveReachablePort(pgBinPath)
	if err != nil {
		return err
	}
	_, err = executePSQLOnPort(pgBinPath, port, User, Database, query, false)
	return err
}

// ExecuteAsUser executes a psql command as a specific user without password (for trust auth)
// This is used when trust authentication is temporarily enabled
func ExecuteAsUser(pgBinPath, username, database, query string) error {
	port, err := resolveReachablePort(pgBinPath)
	if err != nil {
		return err
	}
	_, err = executePSQLOnPort(pgBinPath, port, username, database, query, false)
	return err
}

// TestConnection tests if the PostgreSQL connection is working
func TestConnection(pgBinPath string) error {
	query := "SELECT 1;"
	_, err := executePSQL(pgBinPath, query)
	return err
}

func resolveReachablePort(pgBinPath string) (string, error) {
	var lastErr error
	for _, port := range preferredPorts {
		_, err := executePSQLOnPort(pgBinPath, port, User, Database, "SELECT 1;", true)
		if err == nil {
			return port, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", fmt.Errorf("tidak bisa konek PostgreSQL di port %v: %w", preferredPorts, lastErr)
	}
	return "", fmt.Errorf("tidak bisa konek PostgreSQL di port %v", preferredPorts)
}

func executePSQLOnPort(pgBinPath, port, username, database, query string, tuplesOnly bool) (string, error) {
	psqlPath := fmt.Sprintf("%s\\psql.exe", pgBinPath)

	args := []string{
		"-h", Host,
		"-p", port,
		"-U", username,
		"-d", database,
		"-c", query,
	}
	if tuplesOnly {
		args = append(args, "-t")
	}

	cmd := exec.Command(psqlPath, args...)
	if username == User {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+Password)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("psql error (port %s): %w, stderr: %s", port, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func GetResolvedPortForPreview(pgBinPath string) string {
	port, err := resolveReachablePort(pgBinPath)
	if err != nil {
		return strconv.Itoa(5445)
	}
	return port
}
