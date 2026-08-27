package mcp

import "errors"

var (
	ErrInvalidRequest        = errors.New("invalid MCP request")
	ErrInvalidParams         = errors.New("invalid MCP parameters")
	ErrMethodNotFound        = errors.New("MCP method not found")
	ErrToolNotFound          = errors.New("MCP tool not found")
	ErrResourceNotFound      = errors.New("MCP resource not found")
	ErrDuplicateTool         = errors.New("MCP tool already registered")
	ErrDuplicateResource     = errors.New("MCP resource already registered")
	ErrInvalidTool           = errors.New("invalid MCP tool")
	ErrInvalidResource       = errors.New("invalid MCP resource")
	ErrUnknownScope          = errors.New("unknown MCP scope")
	ErrMissingScope          = errors.New("required MCP scope is missing")
	ErrInvalidToken          = errors.New("invalid MCP bearer token")
	ErrTokenExpired          = errors.New("MCP bearer token expired")
	ErrTokenRevoked          = errors.New("MCP bearer token revoked")
	ErrTokenAlreadyRevealed  = errors.New("MCP token has already been revealed")
	ErrInvalidAuthorization  = errors.New("invalid MCP authorization header")
	ErrReplayDetected        = errors.New("MCP request replay detected")
	ErrIdempotencyConflict   = errors.New("MCP idempotency key conflict")
	ErrIdempotencyInProgress = errors.New("MCP idempotent request is already in progress")
	ErrMissingIdempotencyKey = errors.New("idempotency key is required")
	ErrMissingNonce          = errors.New("nonce is required")
	ErrInvalidNonce          = errors.New("invalid nonce")
	ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")
	ErrUnsupportedVersion    = errors.New("unsupported MCP protocol version")
)

// JSON-RPC error codes used by the transport-neutral MCP adapter.
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603

	// Auth and replay errors use the application-reserved JSON-RPC range.
	UnauthorizedError  = -32001
	NotFoundError      = -32002
	ForbiddenError     = -32003
	ReplayError        = -32004
	IdempotencyError   = -32005
	UnsupportedVersion = -32006
)

// RPCError is a JSON-RPC error object. Data is deliberately optional and
// should contain only non-sensitive, actionable metadata.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`

	cause error
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *RPCError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func newRPCError(code int, message string, cause error, data any) *RPCError {
	return &RPCError{Code: code, Message: message, Data: data, cause: cause}
}
