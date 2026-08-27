package assistant

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidInput           = errors.New("assistant input is invalid")
	ErrNilGenerator           = errors.New("assistant generator is nil")
	ErrNilRetriever           = errors.New("assistant retriever is nil")
	ErrEvidenceNotFound       = errors.New("assistant citation references unknown evidence")
	ErrDuplicateEvidence      = errors.New("assistant evidence contains duplicate identifiers")
	ErrDuplicateCitation      = errors.New("assistant response contains duplicate citations")
	ErrQuoteRequired          = errors.New("assistant citation must include a supporting quote")
	ErrQuoteNotSupported      = errors.New("assistant citation quote is not present in evidence")
	ErrQuoteNotInAnswer       = errors.New("assistant citation quote is not present in answer")
	ErrScopeMismatch          = errors.New("assistant data is outside the authenticated scope")
	ErrDependencyFailure      = errors.New("assistant dependency failed")
	ErrInvalidAction          = errors.New("assistant suggested action is invalid")
	ErrInvalidReceipt         = errors.New("assistant action receipt is invalid")
	ErrReceiptActionMissing   = errors.New("assistant receipt references an action that was not suggested")
	ErrReceiptBindingMismatch = errors.New("assistant action receipt does not match the suggested action")
	ErrUntrustedReceipt       = errors.New("assistant action receipt attachment is not trusted")
)

// ValidationError identifies a deterministic field-level contract failure.
// It deliberately contains no provider payload or source content.
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

func invalidAction(field, reason string) error {
	return fmt.Errorf("%w: %w", ErrInvalidAction, invalidField(field, reason))
}

func invalidReceipt(field, reason string) error {
	return fmt.Errorf("%w: %w", ErrInvalidReceipt, invalidField(field, reason))
}
