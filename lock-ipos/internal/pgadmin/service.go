package pgadmin

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lock-ipos/lock-ipos/internal/logger"
)

// StopPostgreSQLService stops the PostgreSQL service
func StopPostgreSQLService(serviceName string) error {
	logger.Infof("Stopping PostgreSQL service: %s", serviceName)

	// Use double quotes around service name to handle spaces properly
	cmd := fmt.Sprintf("Stop-Service -Name \"%s\" -Force -ErrorAction SilentlyContinue", serviceName)
	output, err := executePowerShellCommand(cmd)
	if err != nil {
		// Check if service is already stopped
		if strings.Contains(output, "already stopped") || strings.Contains(output, "not running") {
			logger.Info("Service is already stopped")
			return nil
		}
		return NewServiceError("stop", serviceName, err)
	}

	logger.Info("PostgreSQL service stopped successfully")
	return nil
}

// StartPostgreSQLService starts the PostgreSQL service
func StartPostgreSQLService(serviceName string) error {
	logger.Infof("Starting PostgreSQL service: %s", serviceName)

	// Use double quotes around service name to handle spaces properly
	cmd := fmt.Sprintf("Start-Service -Name \"%s\" -ErrorAction SilentlyContinue", serviceName)
	output, err := executePowerShellCommand(cmd)
	if err != nil {
		// Check if service is already running
		if strings.Contains(output, "already running") || strings.Contains(output, "already started") {
			logger.Info("Service is already running")
			return nil
		}
		return NewServiceError("start", serviceName, err)
	}

	logger.Info("PostgreSQL service started successfully")
	return nil
}

// RestartPostgreSQLService restarts the PostgreSQL service
func RestartPostgreSQLService(serviceName string) error {
	logger.Infof("Restarting PostgreSQL service: %s", serviceName)

	// First stop
	if err := StopPostgreSQLService(serviceName); err != nil {
		return err
	}

	// Wait a moment for the service to fully stop
	time.Sleep(2 * time.Second)

	// Then start
	if err := StartPostgreSQLService(serviceName); err != nil {
		return err
	}

	logger.Info("PostgreSQL service restarted successfully")
	return nil
}

// executePowerShellCommand executes a PowerShell command and returns the output
func executePowerShellCommand(command string) (string, error) {
	cmd := exec.Command("powershell", "-Command", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
