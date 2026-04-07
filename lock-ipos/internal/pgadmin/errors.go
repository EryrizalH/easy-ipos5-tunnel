package pgadmin

import "fmt"

// PgHbaError represents an error related to pg_hba.conf operations
type PgHbaError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

func (e *PgHbaError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("pg_hba.conf error during %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("pg_hba.conf error during %s", e.Op)
}

func (e *PgHbaError) Unwrap() error {
	return e.Err
}

// NewPgHbaError creates a new PgHbaError
func NewPgHbaError(op string, err error) *PgHbaError {
	return &PgHbaError{Op: op, Err: err}
}

// ServiceError represents an error related to Windows service operations
type ServiceError struct {
	Op        string // Operation that failed
	Service   string // Service name
	Err       error  // Underlying error
}

func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("service error during %s for %s: %v", e.Op, e.Service, e.Err)
	}
	return fmt.Sprintf("service error during %s for %s", e.Op, e.Service)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError creates a new ServiceError
func NewServiceError(op, service string, err error) *ServiceError {
	return &ServiceError{Op: op, Service: service, Err: err}
}
