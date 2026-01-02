package postgres

import "errors"

var (
	// ErrOptimisticLock is returned when an update/delete fails due to version mismatch.
	ErrOptimisticLock = errors.New("userorg/postgres: optimistic locking conflict")
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("userorg/postgres: resource not found")
	// ErrConflict is returned when a resource already exists (duplicate).
	ErrConflict = errors.New("userorg/postgres: resource already exists")
	// ErrForbidden is returned when an operation is not allowed (e.g., modifying builtin resources).
	ErrForbidden = errors.New("userorg/postgres: operation not permitted")
	// ErrNoLimitsSpecified is returned when creating a token policy without any limits.
	ErrNoLimitsSpecified = errors.New("userorg/postgres: at least one limit (1h, 24h, or 7d) must be specified")
)
