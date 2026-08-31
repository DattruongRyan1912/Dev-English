package knowledge

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidInput           = errors.New("knowledge input is invalid")
	ErrNilRepository          = errors.New("knowledge repository is nil")
	ErrWorkspaceMismatch      = errors.New("knowledge entity is outside the workspace scope")
	ErrRevisionImmutable      = errors.New("knowledge source revision is immutable")
	ErrEvidenceRequired       = errors.New("canonical knowledge claim requires evidence")
	ErrCanonicalEvidenceStale = errors.New("canonical knowledge claim has no current evidence")
	ErrStaleEvidence          = errors.New("current knowledge claim cannot use stale or unknown evidence")
	ErrDuplicateEvidence      = errors.New("knowledge claim contains duplicate evidence")
	ErrConflict               = errors.New("knowledge resource already exists")
	ErrNotFound               = errors.New("knowledge resource was not found")
	ErrUnsupportedRead        = errors.New("knowledge read capability is not configured")
)

// ValidationError identifies the first deterministic field-level validation
// failure without exposing provider payloads or source content.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrInvalidInput.Error()
	}
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", ErrInvalidInput, e.Reason)
	}
	return fmt.Sprintf("%s: %s: %s", ErrInvalidInput, e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

func invalidField(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

func requireIdentifier(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalidField(field, "must not be blank")
	}
	if strings.TrimSpace(value) != value {
		return invalidField(field, "must not have leading or trailing whitespace")
	}
	return nil
}

func requireText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalidField(field, "must not be blank")
	}
	return nil
}
