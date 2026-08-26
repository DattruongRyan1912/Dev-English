package work

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound            = errors.New("work record not found")
	ErrAlreadyExists       = errors.New("work record already exists")
	ErrAlreadyTrashed      = errors.New("work record is already trashed")
	ErrNotTrashed          = errors.New("work record is not trashed")
	ErrRecordTrashed       = errors.New("work record is trashed")
	ErrVersionConflict     = errors.New("work record version conflict")
	ErrIdempotencyConflict = errors.New("idempotency key was reused with a different request")
	ErrPurgeNotReady       = errors.New("work record is not old enough to purge")
	ErrDependenciesExist   = errors.New("work record has dependent records")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "validation failed"
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type VersionConflictError struct {
	EntityType EntityType
	EntityID   string
	Expected   int64
	Actual     int64
}

func (e *VersionConflictError) Error() string {
	return fmt.Sprintf("%s %q expected version %d, current version is %d", e.EntityType, e.EntityID, e.Expected, e.Actual)
}

func (e *VersionConflictError) Unwrap() error { return ErrVersionConflict }

// StatusCode lets HTTP adapters map the typed domain error without importing
// net/http into the domain package. Version conflicts are HTTP 409.
func (e *VersionConflictError) StatusCode() int { return 409 }

type IdempotencyConflictError struct {
	Key                string
	ExistingOperation  string
	RequestedOperation string
}

func (e *IdempotencyConflictError) Error() string {
	return fmt.Sprintf("idempotency key %q already belongs to %s, not %s", e.Key, e.ExistingOperation, e.RequestedOperation)
}

func (e *IdempotencyConflictError) Unwrap() error { return ErrIdempotencyConflict }

func (e *IdempotencyConflictError) StatusCode() int { return 409 }

type NotFoundError struct {
	EntityType EntityType
	EntityID   string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.EntityType, e.EntityID)
}

func (e *NotFoundError) Unwrap() error { return ErrNotFound }

type PurgeNotReadyError struct {
	EntityType EntityType
	EntityID   string
	EligibleAt string
}

func (e *PurgeNotReadyError) Error() string {
	return fmt.Sprintf("%s %q can be purged after %s", e.EntityType, e.EntityID, e.EligibleAt)
}

func (e *PurgeNotReadyError) Unwrap() error { return ErrPurgeNotReady }

func (e *PurgeNotReadyError) StatusCode() int { return 422 }

type DependencyError struct {
	EntityType EntityType
	EntityID   string
	Dependents []EntityType
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("%s %q has dependent records: %v", e.EntityType, e.EntityID, e.Dependents)
}

func (e *DependencyError) Unwrap() error { return ErrDependenciesExist }

func (e *DependencyError) StatusCode() int { return 409 }

type StateError struct {
	EntityType EntityType
	EntityID   string
	State      string
	Cause      error
}

func (e *StateError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s %q has invalid state %q", e.EntityType, e.EntityID, e.State)
	}
	return fmt.Sprintf("%s %q has invalid state %q: %v", e.EntityType, e.EntityID, e.State, e.Cause)
}

func (e *StateError) Unwrap() error { return e.Cause }

func validateIdentifier(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Message: "is required"}
	}
	if value != stringTrimmed(value) {
		return &ValidationError{Field: field, Message: "must not have leading or trailing whitespace"}
	}
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return &ValidationError{Field: field, Message: "must not contain whitespace"}
		}
	}
	return nil
}

func stringTrimmed(value string) string {
	start, end := 0, len(value)
	for start < end && value[start] <= ' ' {
		start++
	}
	for end > start && value[end-1] <= ' ' {
		end--
	}
	return value[start:end]
}

func validateText(field, value string, required bool, maxBytes int) error {
	if required && stringTrimmed(value) == "" {
		return &ValidationError{Field: field, Message: "is required"}
	}
	if len(value) > maxBytes {
		return &ValidationError{Field: field, Message: fmt.Sprintf("must be at most %d bytes", maxBytes)}
	}
	if value != stringTrimmed(value) {
		return &ValidationError{Field: field, Message: "must not have leading or trailing whitespace"}
	}
	return nil
}
