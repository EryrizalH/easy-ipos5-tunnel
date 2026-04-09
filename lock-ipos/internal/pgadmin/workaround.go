package pgadmin

import (
	"fmt"

	"github.com/lock-ipos/lock-ipos/internal/db"
	"github.com/lock-ipos/lock-ipos/internal/logger"
)

const (
	// PostgreSQLServiceName is the Windows service name for PostgreSQL
	PostgreSQLServiceName = "iPgSql.9.5"
	// MainpowerUser is the superuser account for executing ALTER USER commands
	MainpowerUser = "mainpower"
)

// ExecuteSQLViaWorkaround executes arbitrary SQL using the same trust-auth workaround
// used for privileged PostgreSQL operations.
func ExecuteSQLViaWorkaround(pgBinPath, databaseName, sql string) error {
	logger.Info("=== Starting SQL Workaround Workflow ===")

	pgHbaPath, err := FindPgHbaConf(pgBinPath)
	if err != nil {
		return fmt.Errorf("failed to find pg_hba.conf: %w", err)
	}

	backupPath, err := BackupPgHbaConf(pgHbaPath)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fileModified := false
	if err := SetAuthMethodToTrust(pgHbaPath); err != nil {
		return fmt.Errorf("failed to set auth method to trust: %w", err)
	}
	fileModified = true

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Panic recovered in SQL workaround workflow: %v", r)
		}

		if fileModified {
			logger.Info("=== Starting SQL Workaround Cleanup ===")
			if err := RestorePgHbaConf(pgHbaPath, backupPath); err != nil {
				logger.Error("CRITICAL: Failed to restore pg_hba.conf - manual intervention required!")
				logger.Errorf("Restore error: %v", err)
				return
			}

			if err := RestartPostgreSQLService(PostgreSQLServiceName); err != nil {
				logger.Error("CRITICAL: Failed to restart service after restore - manual intervention required!")
				logger.Errorf("Restart error: %v", err)
				return
			}

			logger.Info("=== SQL Workaround Cleanup Completed Successfully ===")
		}
	}()

	logger.Info("Restarting PostgreSQL service to apply trust authentication...")
	if err := StopPostgreSQLService(PostgreSQLServiceName); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	if err := StartPostgreSQLService(PostgreSQLServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	logger.Infof("Executing SQL as %s on database %s...", MainpowerUser, databaseName)
	if err := db.ExecuteAsUser(pgBinPath, MainpowerUser, databaseName, sql); err != nil {
		return fmt.Errorf("failed to execute SQL as mainpower: %w", err)
	}

	logger.Info("=== SQL Workaround Workflow Completed Successfully ===")
	logger.Info("Cleanup (restore + restart) will execute via defer")
	return nil
}

// SetPermissionViaWorkaround sets the PostgreSQL user permission using the trust authentication workaround
func SetPermissionViaWorkaround(pgBinPath, targetUser string, allowCreateDB bool) error {
	logger.Info("=== Starting Permission Workaround Workflow ===")

	// Step 1: Find pg_hba.conf
	pgHbaPath, err := FindPgHbaConf(pgBinPath)
	if err != nil {
		return fmt.Errorf("failed to find pg_hba.conf: %w", err)
	}

	// Step 2: Create backup
	backupPath, err := BackupPgHbaConf(pgHbaPath)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fileModified := false

	// Step 3: Modify to trust
	if err := SetAuthMethodToTrust(pgHbaPath); err != nil {
		return fmt.Errorf("failed to set auth method to trust: %w", err)
	}
	fileModified = true

	// CRITICAL: Always restore, even on error
	// This defer ensures cleanup happens even if panic occurs
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Panic recovered in workaround workflow: %v", r)
		}

		if fileModified {
			logger.Info("=== Starting Cleanup ===")
			if err := RestorePgHbaConf(pgHbaPath, backupPath); err != nil {
				logger.Error("CRITICAL: Failed to restore pg_hba.conf - manual intervention required!")
				logger.Errorf("Restore error: %v", err)
				return
			}

			if err := RestartPostgreSQLService(PostgreSQLServiceName); err != nil {
				logger.Error("CRITICAL: Failed to restart service after restore - manual intervention required!")
				logger.Errorf("Restart error: %v", err)
				return
			}

			logger.Info("=== Cleanup Completed Successfully ===")
		}
	}()

	// Step 4: Restart service to apply trust authentication
	logger.Info("Restarting PostgreSQL service to apply trust authentication...")
	if err := StopPostgreSQLService(PostgreSQLServiceName); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	if err := StartPostgreSQLService(PostgreSQLServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Step 5: Execute ALTER USER as mainpower (no password needed with trust)
	logger.Infof("Executing ALTER USER %s as %s...", targetUser, MainpowerUser)
	if err := executeAsMainpower(pgBinPath, targetUser, allowCreateDB); err != nil {
		return fmt.Errorf("failed to execute ALTER USER as mainpower: %w", err)
	}

	// Step 6: Verify the change
	logger.Info("Verifying permission change...")
	canCreateDB, err := db.GetCurrentPermission(pgBinPath)
	if err != nil {
		logger.Warnf("Could not verify permission (connection may not be restored yet): %v", err)
	} else {
		if canCreateDB == allowCreateDB {
			logger.Infof("Verification successful: Permission is now set to %v", allowCreateDB)
		} else {
			logger.Warnf("Verification mismatch: Expected %v, got %v", allowCreateDB, canCreateDB)
		}
	}

	logger.Info("=== Workaround Workflow Completed Successfully ===")
	logger.Info("Cleanup (restore + restart) will execute via defer")

	// Step 7: Cleanup via defer (restore + restart) happens after return
	return nil
}

// executeAsMainpower executes an ALTER USER command as the mainpower superuser
func executeAsMainpower(pgBinPath, username string, allowCreateDB bool) error {
	var query string
	if allowCreateDB {
		query = fmt.Sprintf("ALTER USER %s CREATEDB;", username)
	} else {
		query = fmt.Sprintf("ALTER USER %s NOCREATEDB;", username)
	}

	logger.Infof("Executing as %s: %s", MainpowerUser, query)

	// Execute using db.ExecuteAsUser (which uses trust auth)
	if err := db.ExecuteAsUser(pgBinPath, MainpowerUser, "postgres", query); err != nil {
		return fmt.Errorf("failed to execute as mainpower: %w", err)
	}

	logger.Info("ALTER USER command executed successfully")
	return nil
}
