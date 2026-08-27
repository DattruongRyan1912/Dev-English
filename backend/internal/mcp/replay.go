package mcp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	DefaultReplayTTL      = 5 * time.Minute
	maxNonceLength        = 256
	maxIdempotencyKeySize = 256
)

type ReplayGuardOption func(*ReplayGuard)

func WithReplayClock(clock func() time.Time) ReplayGuardOption {
	return func(guard *ReplayGuard) {
		if clock != nil {
			guard.clock = clock
		}
	}
}

func WithReplayTTL(ttl time.Duration) ReplayGuardOption {
	return func(guard *ReplayGuard) {
		if ttl > 0 {
			guard.ttl = ttl
		}
	}
}

type idempotencyRecord struct {
	actionHash [sha256.Size]byte
	response   []byte
	expiresAt  time.Time
	pending    bool
}

// ReplayGuard prevents a nonce from being accepted twice and makes an
// idempotency key safe under concurrent delivery. It stores response bytes,
// not request secrets.
type ReplayGuard struct {
	mu          sync.Mutex
	nonces      map[string]time.Time
	idempotency map[string]idempotencyRecord
	clock       func() time.Time
	ttl         time.Duration
}

func NewReplayGuard(options ...ReplayGuardOption) *ReplayGuard {
	guard := &ReplayGuard{
		nonces:      make(map[string]time.Time),
		idempotency: make(map[string]idempotencyRecord),
		clock:       func() time.Time { return time.Now().UTC() },
		ttl:         DefaultReplayTTL,
	}
	for _, option := range options {
		if option != nil {
			option(guard)
		}
	}
	return guard
}

func (guard *ReplayGuard) now() time.Time {
	if guard.clock == nil {
		return time.Now().UTC()
	}
	return guard.clock().UTC()
}

// RecordNonce consumes a nonce for one token identity. A nonce is intentionally
// scoped to a bearer token, so two separate MCP credentials cannot collide.
func (guard *ReplayGuard) RecordNonce(subject, nonce string, now time.Time) error {
	if strings.TrimSpace(subject) == "" {
		return ErrInvalidNonce
	}
	if err := validateNonce(nonce); err != nil {
		return err
	}
	if now.IsZero() {
		now = guard.now()
	}

	guard.mu.Lock()
	defer guard.mu.Unlock()
	guard.pruneLocked(now)
	key := subject + "\x00" + nonce
	if _, exists := guard.nonces[key]; exists {
		return ErrReplayDetected
	}
	guard.nonces[key] = now.Add(guard.ttl)
	return nil
}

// BeginIdempotency reserves a key. If the exact same action already completed,
// it returns the original response so a retry cannot execute the action twice.
func (guard *ReplayGuard) BeginIdempotency(subject, key string, actionHash [sha256.Size]byte, now time.Time) ([]byte, bool, error) {
	if strings.TrimSpace(subject) == "" {
		return nil, false, ErrInvalidIdempotencyKey
	}
	if key == "" {
		return nil, false, nil
	}
	if err := validateIdempotencyKey(key); err != nil {
		return nil, false, err
	}
	if now.IsZero() {
		now = guard.now()
	}

	guard.mu.Lock()
	defer guard.mu.Unlock()
	guard.pruneLocked(now)
	mapKey := subject + "\x00" + key
	record, exists := guard.idempotency[mapKey]
	if !exists {
		guard.idempotency[mapKey] = idempotencyRecord{
			actionHash: actionHash,
			expiresAt:  now.Add(guard.ttl),
			pending:    true,
		}
		return nil, false, nil
	}
	if !hmac.Equal(record.actionHash[:], actionHash[:]) {
		return nil, false, ErrIdempotencyConflict
	}
	if record.pending {
		return nil, false, ErrIdempotencyInProgress
	}
	return append([]byte(nil), record.response...), true, nil
}

func (guard *ReplayGuard) CompleteIdempotency(subject, key string, actionHash [sha256.Size]byte, response []byte, now time.Time) error {
	if strings.TrimSpace(subject) == "" {
		return ErrInvalidIdempotencyKey
	}
	if err := validateIdempotencyKey(key); err != nil {
		return err
	}
	if now.IsZero() {
		now = guard.now()
	}

	guard.mu.Lock()
	defer guard.mu.Unlock()
	guard.pruneLocked(now)
	mapKey := subject + "\x00" + key
	record, exists := guard.idempotency[mapKey]
	if !exists {
		return ErrInvalidIdempotencyKey
	}
	if !hmac.Equal(record.actionHash[:], actionHash[:]) {
		return ErrIdempotencyConflict
	}
	if !record.pending {
		return nil
	}
	record.pending = false
	record.response = append([]byte(nil), response...)
	record.expiresAt = now.Add(guard.ttl)
	guard.idempotency[mapKey] = record
	return nil
}

func (guard *ReplayGuard) AbortIdempotency(subject, key string, actionHash [sha256.Size]byte) error {
	if strings.TrimSpace(subject) == "" {
		return ErrInvalidIdempotencyKey
	}
	if err := validateIdempotencyKey(key); err != nil {
		return err
	}
	guard.mu.Lock()
	defer guard.mu.Unlock()
	mapKey := subject + "\x00" + key
	record, exists := guard.idempotency[mapKey]
	if !exists {
		return nil
	}
	if !hmac.Equal(record.actionHash[:], actionHash[:]) {
		return ErrIdempotencyConflict
	}
	if record.pending {
		delete(guard.idempotency, mapKey)
	}
	return nil
}

func (guard *ReplayGuard) pruneLocked(now time.Time) {
	for key, expiresAt := range guard.nonces {
		if !now.Before(expiresAt) {
			delete(guard.nonces, key)
		}
	}
	for key, record := range guard.idempotency {
		if !now.Before(record.expiresAt) {
			delete(guard.idempotency, key)
		}
	}
}

func validateNonce(nonce string) error {
	if nonce == "" || len(nonce) > maxNonceLength || strings.TrimSpace(nonce) == "" {
		return ErrInvalidNonce
	}
	if strings.IndexFunc(nonce, func(r rune) bool { return r == '\r' || r == '\n' || r == '\x00' || r == '\t' }) >= 0 {
		return ErrInvalidNonce
	}
	return nil
}

func validateIdempotencyKey(key string) error {
	if key == "" || len(key) > maxIdempotencyKeySize || strings.TrimSpace(key) == "" {
		return ErrInvalidIdempotencyKey
	}
	if strings.IndexFunc(key, func(r rune) bool { return r == '\r' || r == '\n' || r == '\x00' || r == '\t' }) >= 0 {
		return ErrInvalidIdempotencyKey
	}
	return nil
}

// CanonicalJSON normalizes JSON before an action hash is calculated. This
// makes harmless whitespace or object-key formatting changes equivalent while
// keeping a different action bound to a different hash.
func CanonicalJSON(raw []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return []byte("null"), nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, ErrInvalidRequest
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, ErrInvalidRequest
	} else if !errors.Is(err, io.EOF) {
		// The decoder's second decode must reach EOF. Keep this branch explicit
		// without exposing parser details to callers.
		return nil, ErrInvalidRequest
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	return canonical, nil
}

// ActionHash creates the idempotency binding for one JSON-RPC method and its
// parameters. It contains no bearer token or other credential.
func ActionHash(method string, params json.RawMessage) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	method = strings.TrimSpace(method)
	if method == "" {
		return result, ErrInvalidRequest
	}
	canonical, err := CanonicalJSON(params)
	if err != nil {
		return result, err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(method))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(canonical)
	copy(result[:], hash.Sum(nil))
	return result, nil
}
