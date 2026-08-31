// Package connectors defines provider-neutral boundaries for external data.
// Implementations belong outside this package and must keep provider secrets
// out of errors and test output.
package connectors

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var (
	ErrInvalidCursor           = errors.New("connector cursor is invalid")
	ErrInvalidRevisionIdentity = errors.New("connector revision identity is invalid")
	ErrMissingIdempotencyKey   = errors.New("safe write idempotency key is required")
	ErrInvalidWorkspaceID      = errors.New("connector workspace ID is invalid")
	ErrInvalidRepository       = errors.New("connector repository is invalid")
	ErrInvalidPageSize         = errors.New("connector page size is invalid")
	ErrRevisionAlreadyExists   = errors.New("connector revision already exists")
	ErrInvalidSyncState        = errors.New("connector sync state is invalid")
	ErrNotFound                = errors.New("connector resource not found")
	ErrWriteConflict           = errors.New("connector write revision conflict")
	ErrInvalidWriteTarget      = errors.New("safe write target is invalid")
	ErrUnsupportedWrite        = errors.New("safe write operation is not supported")
	ErrMissingUserID           = errors.New("safe write user ID is required")
	ErrMissingChallenge        = errors.New("safe write challenge is required")
	ErrInvalidChallenge        = errors.New("safe write challenge is invalid")
	ErrChallengeExpired        = errors.New("safe write challenge has expired")
	ErrChallengeUsed           = errors.New("safe write challenge has already been used")
	ErrIdempotencyConflict     = errors.New("safe write idempotency key conflicts with another action")
	ErrInvalidProviderPayload  = errors.New("provider returned an invalid payload")
	ErrReceiptPersistence      = errors.New("safe write receipt could not be persisted")
	ErrReceiptUncertain        = errors.New("safe write outcome is uncertain; reconcile the pending receipt")
	ErrRemovalNotSupported     = errors.New("connector removal handling is not supported")
)

// RetryClass is the bounded retry decision for a provider failure.
type RetryClass string

const (
	RetryNone           RetryClass = "none"
	RetryTransient      RetryClass = "transient"
	RetryRateLimited    RetryClass = "rate_limited"
	RetryAuthentication RetryClass = "authentication"
	RetryAuthorization  RetryClass = "authorization"
	RetryInvalidRequest RetryClass = "invalid_request"
	RetryNotFound       RetryClass = "not_found"
	RetryConflict       RetryClass = "conflict"
	RetryCanceled       RetryClass = "canceled"
)

func (c RetryClass) Retryable() bool {
	return c == RetryTransient || c == RetryRateLimited
}

func ClassifyHTTPStatus(status int) RetryClass {
	switch {
	case status == http.StatusRequestTimeout || status == http.StatusTooEarly:
		return RetryTransient
	case status == http.StatusTooManyRequests:
		return RetryRateLimited
	case status == http.StatusUnauthorized:
		return RetryAuthentication
	case status == http.StatusForbidden:
		return RetryAuthorization
	case status == http.StatusNotFound:
		return RetryNotFound
	case status == http.StatusConflict:
		return RetryConflict
	case status >= http.StatusInternalServerError && status <= 599:
		return RetryTransient
	case status >= http.StatusBadRequest && status <= 499:
		return RetryInvalidRequest
	default:
		return RetryNone
	}
}

// ProviderError is safe to log. Detail is redacted on construction and when
// formatted; the original provider response body is never exposed.
type ProviderError struct {
	Provider   string
	Operation  string
	StatusCode int
	Class      RetryClass
	RequestID  string
	detail     string
	cause      error
}

func NewProviderError(provider, operation string, status int, detail string) *ProviderError {
	return &ProviderError{
		Provider:   strings.TrimSpace(provider),
		Operation:  strings.TrimSpace(operation),
		StatusCode: status,
		Class:      ClassifyHTTPStatus(status),
		detail:     RedactSecrets(detail),
	}
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "provider error"
	}
	provider := e.Provider
	if provider == "" {
		provider = "provider"
	}
	message := provider
	if e.Operation != "" {
		message += " " + e.Operation
	}
	if e.StatusCode > 0 {
		message += fmt.Sprintf(" returned HTTP %d", e.StatusCode)
	}
	if detail := strings.TrimSpace(RedactSecrets(e.detail)); detail != "" {
		message += ": " + detail
	}
	return message
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *ProviderError) RetryClass() RetryClass {
	if e == nil {
		return RetryNone
	}
	if e.Class != "" {
		return e.Class
	}
	return ClassifyHTTPStatus(e.StatusCode)
}

func ClassifyError(err error) RetryClass {
	if err == nil {
		return RetryNone
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return RetryCanceled
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.RetryClass()
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		return RetryTransient
	}
	return RetryNone
}

func IsRetryable(err error) bool { return ClassifyError(err).Retryable() }

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer|token|basic)\s+)[^\s,;]+`),
	regexp.MustCompile(`(?i)(\b(?:access[_-]?token|api[_-]?key|client[_-]?secret|password|token)\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;&]+)`),
	regexp.MustCompile(`(?i)\b(?:gh[pousr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9_-]+)\b`),
}

// RedactSecrets removes common credential representations before a message
// crosses a logging boundary. It is intentionally conservative and is not a
// substitute for avoiding secret values in provider errors altogether.
func RedactSecrets(value string) string {
	redacted := value
	for index, pattern := range secretPatterns {
		if index < 2 {
			redacted = pattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
			continue
		}
		redacted = pattern.ReplaceAllString(redacted, `[REDACTED]`)
	}
	return redacted
}
