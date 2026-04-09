package pgadmin

import (
	"fmt"

	"github.com/lock-ipos/lock-ipos/internal/db"
	"github.com/lock-ipos/lock-ipos/internal/logger"
	"github.com/lock-ipos/lock-ipos/internal/progress"
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
	return ExecuteSQLViaWorkaroundWithProgress(pgBinPath, databaseName, sql, progress.NopReporter())
}

func ExecuteSQLViaWorkaroundWithProgress(pgBinPath, databaseName, sql string, reporter progress.Reporter) (err error) {
	reporter = normalizeReporter(reporter)
	logger.Info("=== Starting SQL Workaround Workflow ===")
	reporter.Log("Memulai workflow SQL workaround PostgreSQL")

	reporter.StartStep("find-pghba", "Mencari pg_hba.conf")
	pgHbaPath, err := FindPgHbaConf(pgBinPath)
	if err != nil {
		reporter.FinishStep("find-pghba", false, err.Error())
		return fmt.Errorf("failed to find pg_hba.conf: %w", err)
	}
	reporter.Log("pg_hba.conf ditemukan: " + pgHbaPath)
	reporter.FinishStep("find-pghba", true, "Lokasi konfigurasi ditemukan")

	reporter.StartStep("backup-pghba", "Membuat backup pg_hba.conf")
	backupPath, err := BackupPgHbaConf(pgHbaPath)
	if err != nil {
		reporter.FinishStep("backup-pghba", false, err.Error())
		return fmt.Errorf("failed to create backup: %w", err)
	}
	reporter.Log("Backup dibuat: " + backupPath)
	reporter.FinishStep("backup-pghba", true, "Backup selesai dibuat")

	cleanupNeeded := false
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Panic recovered in SQL workaround workflow: %v", r)
			reporter.Log(fmt.Sprintf("Panic workflow: %v", r))
			if err == nil {
				err = fmt.Errorf("panic pada workflow SQL workaround: %v", r)
			}
		}

		if !cleanupNeeded {
			return
		}

		logger.Info("=== Starting SQL Workaround Cleanup ===")
		reporter.StartStep("restore-pghba", "Mengembalikan pg_hba.conf")
		if restoreErr := RestorePgHbaConf(pgHbaPath, backupPath); restoreErr != nil {
			reporter.FinishStep("restore-pghba", false, restoreErr.Error())
			reporter.Log("CRITICAL: restore pg_hba.conf gagal")
			if err == nil {
				err = fmt.Errorf("failed to restore pg_hba.conf: %w", restoreErr)
			}
			return
		}
		reporter.FinishStep("restore-pghba", true, "Konfigurasi autentikasi dikembalikan")

		reporter.StartStep("restart-cleanup", "Restart PostgreSQL untuk cleanup")
		if restartErr := RestartPostgreSQLService(PostgreSQLServiceName); restartErr != nil {
			reporter.FinishStep("restart-cleanup", false, restartErr.Error())
			reporter.Log("CRITICAL: restart PostgreSQL setelah restore gagal")
			if err == nil {
				err = fmt.Errorf("failed to restart service after restore: %w", restartErr)
			}
			return
		}
		reporter.FinishStep("restart-cleanup", true, "Service PostgreSQL aktif kembali")
		logger.Info("=== SQL Workaround Cleanup Completed Successfully ===")
	}()

	reporter.StartStep("set-trust", "Mengubah auth menjadi trust")
	if err := SetAuthMethodToTrust(pgHbaPath); err != nil {
		reporter.FinishStep("set-trust", false, err.Error())
		return fmt.Errorf("failed to set auth method to trust: %w", err)
	}
	cleanupNeeded = true
	reporter.FinishStep("set-trust", true, "Autentikasi trust aktif sementara")

	logger.Info("Restarting PostgreSQL service to apply trust authentication...")
	reporter.StartStep("restart-trust", "Restart PostgreSQL untuk mode trust")
	if err := StopPostgreSQLService(PostgreSQLServiceName); err != nil {
		reporter.FinishStep("restart-trust", false, err.Error())
		return fmt.Errorf("failed to stop service: %w", err)
	}
	if err := StartPostgreSQLService(PostgreSQLServiceName); err != nil {
		reporter.FinishStep("restart-trust", false, err.Error())
		return fmt.Errorf("failed to start service: %w", err)
	}
	reporter.FinishStep("restart-trust", true, "Mode trust siap digunakan")

	logger.Infof("Executing SQL as %s on database %s...", MainpowerUser, databaseName)
	reporter.StartStep("execute-sql", "Menjalankan SQL privileged")
	if err := db.ExecuteAsUser(pgBinPath, MainpowerUser, databaseName, sql); err != nil {
		reporter.FinishStep("execute-sql", false, err.Error())
		return fmt.Errorf("failed to execute SQL as mainpower: %w", err)
	}
	reporter.FinishStep("execute-sql", true, "SQL berhasil dijalankan")

	logger.Info("=== SQL Workaround Workflow Completed Successfully ===")
	logger.Info("Cleanup (restore + restart) will execute via defer")
	reporter.Log("SQL workaround selesai, menjalankan cleanup restore + restart")
	return nil
}

// SetPermissionViaWorkaround sets the PostgreSQL user permission using the trust authentication workaround
func SetPermissionViaWorkaround(pgBinPath, targetUser string, allowCreateDB bool) error {
	return SetPermissionViaWorkaroundWithProgress(pgBinPath, targetUser, allowCreateDB, progress.NopReporter())
}

func SetPermissionViaWorkaroundWithProgress(pgBinPath, targetUser string, allowCreateDB bool, reporter progress.Reporter) (err error) {
	reporter = normalizeReporter(reporter)
	logger.Info("=== Starting Permission Workaround Workflow ===")
	reporter.Log("Memulai workflow perubahan permission PostgreSQL")

	reporter.StartStep("find-pghba", "Mencari pg_hba.conf")
	pgHbaPath, err := FindPgHbaConf(pgBinPath)
	if err != nil {
		reporter.FinishStep("find-pghba", false, err.Error())
		return fmt.Errorf("failed to find pg_hba.conf: %w", err)
	}
	reporter.Log("pg_hba.conf ditemukan: " + pgHbaPath)
	reporter.FinishStep("find-pghba", true, "Lokasi konfigurasi ditemukan")

	reporter.StartStep("backup-pghba", "Membuat backup pg_hba.conf")
	backupPath, err := BackupPgHbaConf(pgHbaPath)
	if err != nil {
		reporter.FinishStep("backup-pghba", false, err.Error())
		return fmt.Errorf("failed to create backup: %w", err)
	}
	reporter.Log("Backup dibuat: " + backupPath)
	reporter.FinishStep("backup-pghba", true, "Backup siap dipakai untuk rollback")

	cleanupNeeded := false
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Panic recovered in workaround workflow: %v", r)
			reporter.Log(fmt.Sprintf("Panic workflow: %v", r))
			if err == nil {
				err = fmt.Errorf("panic pada workflow permission workaround: %v", r)
			}
		}

		if !cleanupNeeded {
			return
		}

		logger.Info("=== Starting Cleanup ===")
		reporter.StartStep("restore-pghba", "Mengembalikan pg_hba.conf")
		if restoreErr := RestorePgHbaConf(pgHbaPath, backupPath); restoreErr != nil {
			logger.Error("CRITICAL: Failed to restore pg_hba.conf - manual intervention required!")
			logger.Errorf("Restore error: %v", restoreErr)
			reporter.FinishStep("restore-pghba", false, restoreErr.Error())
			reporter.Log("CRITICAL: restore pg_hba.conf gagal")
			if err == nil {
				err = fmt.Errorf("failed to restore pg_hba.conf: %w", restoreErr)
			}
			return
		}
		reporter.FinishStep("restore-pghba", true, "Konfigurasi autentikasi dikembalikan")

		reporter.StartStep("restart-cleanup", "Restart PostgreSQL untuk cleanup")
		if restartErr := RestartPostgreSQLService(PostgreSQLServiceName); restartErr != nil {
			logger.Error("CRITICAL: Failed to restart service after restore - manual intervention required!")
			logger.Errorf("Restart error: %v", restartErr)
			reporter.FinishStep("restart-cleanup", false, restartErr.Error())
			reporter.Log("CRITICAL: restart PostgreSQL setelah restore gagal")
			if err == nil {
				err = fmt.Errorf("failed to restart service after restore: %w", restartErr)
			}
			return
		}
		reporter.FinishStep("restart-cleanup", true, "Service PostgreSQL aktif kembali")
		logger.Info("=== Cleanup Completed Successfully ===")
	}()

	reporter.StartStep("set-trust", "Mengubah auth menjadi trust")
	if err := SetAuthMethodToTrust(pgHbaPath); err != nil {
		reporter.FinishStep("set-trust", false, err.Error())
		return fmt.Errorf("failed to set auth method to trust: %w", err)
	}
	cleanupNeeded = true
	reporter.FinishStep("set-trust", true, "Autentikasi trust aktif sementara")

	logger.Info("Restarting PostgreSQL service to apply trust authentication...")
	reporter.StartStep("restart-trust", "Restart PostgreSQL untuk mode trust")
	if err := StopPostgreSQLService(PostgreSQLServiceName); err != nil {
		reporter.FinishStep("restart-trust", false, err.Error())
		return fmt.Errorf("failed to stop service: %w", err)
	}
	if err := StartPostgreSQLService(PostgreSQLServiceName); err != nil {
		reporter.FinishStep("restart-trust", false, err.Error())
		return fmt.Errorf("failed to start service: %w", err)
	}
	reporter.FinishStep("restart-trust", true, "Mode trust siap digunakan")

	stepLabel := "Menjalankan ALTER USER NOCREATEDB"
	if allowCreateDB {
		stepLabel = "Menjalankan ALTER USER CREATEDB"
	}
	logger.Infof("Executing ALTER USER %s as %s...", targetUser, MainpowerUser)
	reporter.StartStep("alter-user", stepLabel)
	if err := executeAsMainpower(pgBinPath, targetUser, allowCreateDB); err != nil {
		reporter.FinishStep("alter-user", false, err.Error())
		return fmt.Errorf("failed to execute ALTER USER as mainpower: %w", err)
	}
	reporter.FinishStep("alter-user", true, "Permission PostgreSQL berhasil diubah")

	logger.Info("Verifying permission change...")
	reporter.StartStep("verify-permission", "Verifikasi permission database")
	canCreateDB, verifyErr := db.GetCurrentPermission(pgBinPath)
	if verifyErr != nil {
		logger.Warnf("Could not verify permission (connection may not be restored yet): %v", verifyErr)
		reporter.FinishStep("verify-permission", true, "Verifikasi akhir akan dicek ulang setelah cleanup")
	} else if canCreateDB == allowCreateDB {
		logger.Infof("Verification successful: Permission is now set to %v", allowCreateDB)
		reporter.FinishStep("verify-permission", true, "Permission sesuai target")
	} else {
		logger.Warnf("Verification mismatch: Expected %v, got %v", allowCreateDB, canCreateDB)
		reporter.FinishStep("verify-permission", false, fmt.Sprintf("Mismatch: target=%v aktual=%v", allowCreateDB, canCreateDB))
		return fmt.Errorf("verification mismatch: expected %v, got %v", allowCreateDB, canCreateDB)
	}

	logger.Info("=== Workaround Workflow Completed Successfully ===")
	logger.Info("Cleanup (restore + restart) will execute via defer")
	reporter.Log("Perubahan permission selesai, menjalankan cleanup restore + restart")
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

func normalizeReporter(reporter progress.Reporter) progress.Reporter {
	if reporter == nil {
		return progress.NopReporter()
	}
	return reporter
}
