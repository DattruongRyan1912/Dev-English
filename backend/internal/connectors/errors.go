// Package connectors defines provider-neutral boundaries for external data.
// Implementations belong outside this package and must keep provider secrets
// out of errors and test output.
package connectors

import (
	"context"
	"encoding/json"
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
	ErrWritePending            = errors.New("safe write is already pending")
	ErrWriteUncertain          = errors.New("safe write outcome is uncertain")
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

// IsRetryable reports whether a class may be retried without changing the
// request. Conflicts require a fresh read and are not blindly retried.
func (c RetryClass) IsRetryable() bool {
	return c == RetryTransient || c == RetryRateLimited
}

// Retryable is the canonical readable alias for IsRetryable.
func (c RetryClass) Retryable() bool {
	return c.IsRetryable()
}

// ClassifyHTTPStatus maps provider HTTP status codes to safe retry classes.
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

// ClassifyHTTPRetry is kept for callers of the earlier connector draft.
func ClassifyHTTPRetry(status int) RetryClass {
	return ClassifyHTTPStatus(status)
}

// ProviderError is a provider-safe error envelope. Provider response detail
// is held privately, redacted at construction and formatting boundaries, and
// deliberately omitted from JSON serialization.
type ProviderError struct {
	Provider   string     `json:"provider"`
	Operation  string     `json:"operation"`
	StatusCode int        `json:"statusCode,omitempty"`
	Class      RetryClass `json:"class,omitempty"`
	RequestID  string     `json:"requestId,omitempty"`
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

// NewHTTPProviderError is an explicit alias for callers handling HTTP APIs.
func NewHTTPProviderError(provider, operation string, status int, detail string) *ProviderError {
	return NewProviderError(provider, operation, status, detail)
}

// WrapProviderError adds provider-safe context while preserving standard
// errors.Is/errors.As matching through the original cause.
func WrapProviderError(provider, operation string, status int, cause error) *ProviderError {
	result := NewProviderError(provider, operation, status, "")
	result.cause = cause
	if cause != nil {
		if class := ClassifyRetry(cause); class != RetryNone {
			result.Class = class
		}
		result.detail = RedactSecrets(cause.Error())
	}
	return result
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "provider error"
	}
	provider := RedactSecrets(strings.TrimSpace(e.Provider))
	if provider == "" {
		provider = "provider"
	}
	message := provider
	if operation := RedactSecrets(strings.TrimSpace(e.Operation)); operation != "" {
		message += " " + operation
	}
	if e.StatusCode > 0 {
		message += fmt.Sprintf(" returned HTTP %d", e.StatusCode)
	}
	if detail := strings.TrimSpace(RedactSecrets(e.detail)); detail != "" {
		message += ": " + detail
	}
	return message
}

// Format prevents fmt's %+v struct formatting from exposing private detail.
func (e *ProviderError) Format(state fmt.State, verb rune) {
	message := e.Error()
	switch verb {
	case 'q':
		fmt.Fprintf(state, "%q", message)
	case 'x':
		fmt.Fprintf(state, "%x", message)
	case 'X':
		fmt.Fprintf(state, "%X", message)
	default:
		fmt.Fprint(state, message)
	}
}

// MarshalJSON serializes only the safe envelope. In particular, detail and
// cause are never traversed by encoding/json.
func (e *ProviderError) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	type safeProviderError struct {
		Provider   string     `json:"provider"`
		Operation  string     `json:"operation"`
		StatusCode int        `json:"statusCode,omitempty"`
		Class      RetryClass `json:"class,omitempty"`
		RequestID  string     `json:"requestId,omitempty"`
	}
	return json.Marshal(safeProviderError{
		Provider:   RedactSecrets(strings.TrimSpace(e.Provider)),
		Operation:  RedactSecrets(strings.TrimSpace(e.Operation)),
		StatusCode: e.StatusCode,
		Class:      e.RetryClass(),
		RequestID:  RedactSecrets(strings.TrimSpace(e.RequestID)),
	})
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

// Retryable reports whether this provider failure may be retried.
func (e *ProviderError) Retryable() bool {
	return e != nil && e.RetryClass().IsRetryable()
}

// ClassifyRetry classifies an error without exposing provider detail.
func ClassifyRetry(err error) RetryClass {
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

// ClassifyError is the canonical name retained for the connector contract.
func ClassifyError(err error) RetryClass {
	return ClassifyRetry(err)
}

func IsRetryable(err error) bool {
	return ClassifyRetry(err).IsRetryable()
}

var (
	privateKeyPattern    = regexp.MustCompile(`(?is)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	authorizationPattern = regexp.MustCompile(`(?i)(\b(?:authorization|proxy-authorization)\b\s*["']?\s*[:=]\s*["']?(?:bearer|token|basic)\s+)[^"\s,;}]+`)
	bearerPattern        = regexp.MustCompile(`(?i)(\bbearer\s+)[^"\s,;}]+`)
	doubleQuotedSecret   = regexp.MustCompile(`(?i)(\b(?:access[_-]?token|refresh[_-]?token|api[_-]?key|client[_-]?secret|secret[_-]?key|private[_-]?key|password|passwd|token|credential)\b\s*["']?\s*[:=]\s*")([^"]*)(")`)
	singleQuotedSecret   = regexp.MustCompile(`(?i)(\b(?:access[_-]?token|refresh[_-]?token|api[_-]?key|client[_-]?secret|secret[_-]?key|private[_-]?key|password|passwd|token|credential)\b\s*["']?\s*[:=]\s*')([^']*)(')`)
	unquotedSecret       = regexp.MustCompile(`(?i)(\b(?:access[_-]?token|refresh[_-]?token|api[_-]?key|client[_-]?secret|secret[_-]?key|private[_-]?key|password|passwd|token|credential)\b\s*["']?\s*[:=]\s*)([^\s,;&}"']+)`)
	providerTokenPattern = regexp.MustCompile(`(?i)\b(?:gh[pousr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|ya29\.[A-Za-z0-9._-]+|sk-[A-Za-z0-9_-]+)\b`)
)

// RedactSecrets removes common authorization headers, quoted JSON fields,
// query parameters, token formats and PEM private-key blocks. It never logs
// the value supplied by a provider as raw diagnostic detail.
func RedactSecrets(value string) string {
	redacted := privateKeyPattern.ReplaceAllString(value, "[REDACTED]")
	redacted = authorizationPattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = bearerPattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = doubleQuotedSecret.ReplaceAllString(redacted, `${1}[REDACTED]${3}`)
	redacted = singleQuotedSecret.ReplaceAllString(redacted, `${1}[REDACTED]${3}`)
	redacted = unquotedSecret.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = providerTokenPattern.ReplaceAllString(redacted, "[REDACTED]")
	return redacted
}

// RedactProviderError returns a safe copy while retaining the matching chain.
func RedactProviderError(err error) error {
	if err == nil {
		return nil
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		copyOfError := *providerErr
		copyOfError.Provider = RedactSecrets(copyOfError.Provider)
		copyOfError.Operation = RedactSecrets(copyOfError.Operation)
		copyOfError.RequestID = RedactSecrets(copyOfError.RequestID)
		copyOfError.detail = RedactSecrets(copyOfError.detail)
		return &copyOfError
	}
	return &redactedError{message: RedactSecrets(err.Error()), cause: err}
}

type redactedError struct {
	message string
	cause   error
}

func (e *redactedError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *redactedError) Format(state fmt.State, verb rune) {
	message := e.Error()
	if verb == 'q' {
		fmt.Fprintf(state, "%q", message)
		return
	}
	fmt.Fprint(state, message)
}

func (e *redactedError) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	return json.Marshal(struct {
		Error string `json:"error"`
	}{Error: e.message})
}

func (e *redactedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// RedactErrorMessage is a descriptive alias for RedactSecrets at logging
// boundaries that only need a string.
func RedactErrorMessage(value string) string {
	return RedactSecrets(value)
}
