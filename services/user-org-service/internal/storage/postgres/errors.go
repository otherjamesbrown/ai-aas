package postgres

import "errors"

var (
	// ErrOptimisticLock is returned when an update/delete fails due to version mismatch.
	ErrOptimisticLock = errors.New("userorg/postgres: optimistic locking conflict")
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("userorg/postgres: resource not found")
)
