package models

import "time"

// Config holds the application configuration
type Config struct {
	PgBinPath string
	Host      string
	Port      string
	User      string
	Password  string
	Database  string
}

// PermissionState represents the current permission state
type PermissionState struct {
	CanCreateDB bool
	LastUpdated time.Time
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		Host:     "localhost",
		Port:     "5444",
		User:     "sysi5adm",
		Password: "u&aV23cc.o82dtr1x89c",
		Database: "postgres",
	}
}

// NewPermissionState creates a new PermissionState
func NewPermissionState(canCreateDB bool) *PermissionState {
	return &PermissionState{
		CanCreateDB: canCreateDB,
		LastUpdated: time.Now(),
	}
}
