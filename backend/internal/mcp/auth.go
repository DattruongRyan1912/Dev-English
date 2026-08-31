package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultTokenTTL        = 90 * 24 * time.Hour
	tokenIDBytes           = 16
	tokenSecretBytes       = 32
	tokenPrefix            = "mcp_"
	maxTokenIdentityLength = 256
)

// ErrInvalidTokenIdentity is returned when an application-bound MCP token is
// issued without a complete, control-safe workspace/user identity.
var ErrInvalidTokenIdentity = errors.New("invalid MCP token identity")

// TokenIdentity is the canonical application identity bound to an MCP token.
// It contains no bearer secret and is copied into the verified Principal.
type TokenIdentity struct {
	WorkspaceID string
	UserID      string
}

// TokenStoreOption configures a TokenStore. The clock and random source are
// injectable only to make expiry and issuance tests deterministic.
type TokenStoreOption func(*TokenStore)

func WithTokenClock(clock func() time.Time) TokenStoreOption {
	return func(store *TokenStore) {
		if clock != nil {
			store.clock = clock
		}
	}
}

func WithTokenTTL(ttl time.Duration) TokenStoreOption {
	return func(store *TokenStore) {
		if ttl > 0 {
			store.ttl = ttl
		}
	}
}

func WithTokenRandom(random io.Reader) TokenStoreOption {
	return func(store *TokenStore) {
		if random != nil {
			store.random = random
		}
	}
}

// PersistedToken is the database-safe representation of an MCP credential.
// Digest contains only the one-way bearer-secret digest; the clear token is
// never handed to a persistence implementation.
type PersistedToken struct {
	ID          string
	Digest      []byte
	Scopes      []Scope
	ExpiresAt   time.Time
	Revoked     bool
	WorkspaceID string
	UserID      string
}

// TokenPersistence makes the token lifecycle survive process restarts and
// keeps revocation visible across backend instances.
type TokenPersistence interface {
	Put(context.Context, PersistedToken) error
	Get(context.Context, string) (PersistedToken, bool, error)
	Revoke(context.Context, string, time.Time) error
}

func WithTokenPersistence(persistence TokenPersistence) TokenStoreOption {
	return func(store *TokenStore) {
		store.persistence = persistence
	}
}

type tokenRecord struct {
	digest      [sha256.Size]byte
	scopes      []Scope
	expiresAt   time.Time
	revoked     bool
	workspaceID string
	userID      string
}

// TokenStore stores only a digest of the bearer secret. The clear token is
// returned through IssuedToken.Reveal exactly once and cannot be recovered
// from the store afterwards.
type TokenStore struct {
	mu          sync.RWMutex
	tokens      map[string]tokenRecord
	clock       func() time.Time
	random      io.Reader
	ttl         time.Duration
	persistence TokenPersistence
}

func NewTokenStore(options ...TokenStoreOption) *TokenStore {
	store := &TokenStore{
		tokens: make(map[string]tokenRecord),
		clock:  func() time.Time { return time.Now().UTC() },
		random: rand.Reader,
		ttl:    DefaultTokenTTL,
	}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// IssuedToken contains metadata for an issued credential. The bearer token is
// intentionally private and can only be revealed once to the caller.
type IssuedToken struct {
	ID        string
	Scopes    []Scope
	ExpiresAt time.Time

	state *issuedTokenState
}

type issuedTokenState struct {
	mu       sync.Mutex
	token    string
	revealed bool
}

// Reveal returns the clear bearer token once. A second call, including a
// concurrent call, returns ErrTokenAlreadyRevealed.
func (issued *IssuedToken) Reveal() (string, error) {
	if issued == nil {
		return "", ErrTokenAlreadyRevealed
	}
	state := issued.state
	if state == nil {
		return "", ErrInvalidToken
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.revealed {
		return "", ErrTokenAlreadyRevealed
	}
	state.revealed = true
	token := state.token
	state.token = ""
	if token == "" {
		return "", ErrInvalidToken
	}
	return token, nil
}

// Issue creates a token with the supplied allow-listed scopes. The returned
// token secret is not persisted in clear text.
func (store *TokenStore) Issue(scopes []Scope) (IssuedToken, error) {
	return store.issue(scopes, TokenIdentity{})
}

// IssueForIdentity creates an application-bound token. The lower-level Issue
// API remains available for protocol-only callers that do not need an
// application identity.
func (store *TokenStore) IssueForIdentity(scopes []Scope, identity TokenIdentity) (IssuedToken, error) {
	if err := validateTokenIdentity(identity); err != nil {
		return IssuedToken{}, err
	}
	return store.issue(scopes, identity)
}

func (store *TokenStore) issue(scopes []Scope, identity TokenIdentity) (IssuedToken, error) {
	normalized, err := ParseScopeList(scopeStrings(scopes))
	if err != nil {
		return IssuedToken{}, err
	}
	now := store.now()
	for attempt := 0; attempt < 8; attempt++ {
		idBytes, err := randomBytes(store.random, tokenIDBytes)
		if err != nil {
			return IssuedToken{}, err
		}
		secretBytes, err := randomBytes(store.random, tokenSecretBytes)
		if err != nil {
			return IssuedToken{}, err
		}
		id := base64.RawURLEncoding.EncodeToString(idBytes)
		secret := base64.RawURLEncoding.EncodeToString(secretBytes)
		digest := sha256.Sum256([]byte(secret))
		expiresAt := now.Add(store.ttl)
		record := tokenRecord{
			digest:      digest,
			scopes:      append([]Scope(nil), normalized...),
			expiresAt:   expiresAt,
			workspaceID: identity.WorkspaceID,
			userID:      identity.UserID,
		}
		if store.persistence != nil {
			if err := store.persistence.Put(context.Background(), persistedToken(id, record)); err != nil {
				return IssuedToken{}, ErrTokenPersistence
			}
		}

		store.mu.Lock()
		if _, exists := store.tokens[id]; exists {
			store.mu.Unlock()
			continue
		}
		store.tokens[id] = record
		store.mu.Unlock()

		return IssuedToken{
			ID:        id,
			Scopes:    append([]Scope(nil), normalized...),
			ExpiresAt: expiresAt,
			state:     &issuedTokenState{token: tokenPrefix + id + "." + secret},
		}, nil
	}
	return IssuedToken{}, errors.New("could not allocate a unique MCP token id")
}

// IssueString is a convenience for OAuth-style scope configuration.
func (store *TokenStore) IssueString(scopeString string) (IssuedToken, error) {
	scopes, err := ParseScopes(scopeString)
	if err != nil {
		return IssuedToken{}, err
	}
	return store.Issue(scopes)
}

// Principal is the identity and permission set attached to one MCP request.
// It never contains the bearer secret.
type Principal struct {
	TokenID     string
	Scopes      []Scope
	ExpiresAt   time.Time
	WorkspaceID string
	UserID      string
}

func (principal Principal) HasScope(scope Scope) bool {
	if scope == "" {
		return true
	}
	for _, candidate := range principal.Scopes {
		if candidate == scope {
			return true
		}
	}
	return false
}

func (principal Principal) RequireScope(scope Scope) error {
	if scope == "" {
		return nil
	}
	if principal.HasScope(scope) {
		return nil
	}
	return ErrMissingScope
}

// Verify checks a bearer token at the supplied time. Verification compares
// digests with hmac.Equal rather than a normal byte comparison.
func (store *TokenStore) Verify(token string, now time.Time) (Principal, error) {
	return store.VerifyContext(context.Background(), token, now)
}

// VerifyContext is the request-aware form used by the HTTP transport. It
// allows a persistent store to observe request cancellation while preserving
// the compact Verify API used by protocol/unit callers.
func (store *TokenStore) VerifyContext(ctx context.Context, token string, now time.Time) (Principal, error) {
	id, secret, err := splitToken(token)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	if now.IsZero() {
		now = store.now()
	}
	digest := sha256.Sum256([]byte(secret))

	record, ok, loadErr := store.loadRecord(ctx, id)
	if loadErr != nil || !ok {
		return Principal{}, ErrInvalidToken
	}
	if !hmac.Equal(record.digest[:], digest[:]) {
		return Principal{}, ErrInvalidToken
	}
	if record.revoked {
		return Principal{}, ErrTokenRevoked
	}
	if !now.Before(record.expiresAt) {
		return Principal{}, ErrTokenExpired
	}
	return Principal{
		TokenID:     id,
		Scopes:      append([]Scope(nil), record.scopes...),
		ExpiresAt:   record.expiresAt,
		WorkspaceID: record.workspaceID,
		UserID:      record.userID,
	}, nil
}

func validateTokenIdentity(identity TokenIdentity) error {
	if !validTokenIdentityPart(identity.WorkspaceID) || !validTokenIdentityPart(identity.UserID) {
		return ErrInvalidTokenIdentity
	}
	return nil
}

func validTokenIdentityPart(value string) bool {
	return validMCPIdentifier(value, maxTokenIdentityLength, true)
}

// validMCPIdentifier is the strict identity contract shared by token
// issuance and application argument validation. Downstream services treat
// workspace/user/entity identifiers as trimmed, non-whitespace values and the
// assistant additionally rejects all Unicode control characters; the MCP
// boundary must reject the same values before any dependency is called.
func validMCPIdentifier(value string, maxBytes int, required bool) bool {
	if !required && value == "" {
		return true
	}
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func hasMCPDisallowedControl(value string, allowTextControls bool) bool {
	for _, r := range value {
		if !unicode.IsControl(r) {
			continue
		}
		if allowTextControls && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		return true
	}
	return false
}

func (store *TokenStore) VerifyAuthorization(header string, now time.Time) (Principal, error) {
	return store.VerifyAuthorizationContext(context.Background(), header, now)
}

func (store *TokenStore) VerifyAuthorizationContext(ctx context.Context, header string, now time.Time) (Principal, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Principal{}, ErrInvalidAuthorization
	}
	return store.VerifyContext(ctx, parts[1], now)
}

// Revoke invalidates a token after verifying its secret. The clear token is
// never retained by the store and is not included in any returned error.
func (store *TokenStore) Revoke(token string) error {
	return store.RevokeContext(context.Background(), token)
}

func (store *TokenStore) RevokeContext(ctx context.Context, token string) error {
	id, secret, err := splitToken(token)
	if err != nil {
		return ErrInvalidToken
	}
	digest := sha256.Sum256([]byte(secret))

	record, ok, loadErr := store.loadRecord(ctx, id)
	if loadErr != nil || !ok || !hmac.Equal(record.digest[:], digest[:]) {
		return ErrInvalidToken
	}
	if store.persistence != nil {
		if err := store.persistence.Revoke(ctx, id, store.now()); err != nil {
			return ErrTokenPersistence
		}
	}
	store.mu.Lock()
	record.revoked = true
	store.tokens[id] = record
	store.mu.Unlock()
	return nil
}

// RevokeByID invalidates a token from an authenticated management surface
// without requiring the bearer secret to be sent back to the server. The
// token ID is not a credential and is safe to use as the resource identifier;
// the caller must still be authorized by the transport before invoking this
// method.
func (store *TokenStore) RevokeByID(id string) error {
	return store.RevokeByIDContext(context.Background(), id)
}

func (store *TokenStore) RevokeByIDContext(ctx context.Context, id string) error {
	if store == nil || !validMCPIdentifier(id, maxTokenIdentityLength, true) {
		return ErrInvalidToken
	}
	record, ok, loadErr := store.loadRecord(ctx, id)
	if loadErr != nil {
		return ErrTokenPersistence
	}
	if !ok {
		return ErrInvalidToken
	}
	if store.persistence != nil {
		if err := store.persistence.Revoke(ctx, id, store.now()); err != nil {
			return ErrTokenPersistence
		}
	}
	store.mu.Lock()
	record.revoked = true
	store.tokens[id] = record
	store.mu.Unlock()
	return nil
}

// RevokeByIDForIdentity invalidates a token from an authenticated application
// surface while preserving the token's workspace/user ownership boundary.
// Token IDs are resource identifiers, not bearer credentials, but possession
// of an ID must never allow one user to revoke another user's token.
func (store *TokenStore) RevokeByIDForIdentity(id string, identity TokenIdentity) error {
	return store.RevokeByIDForIdentityContext(context.Background(), id, identity)
}

func (store *TokenStore) RevokeByIDForIdentityContext(ctx context.Context, id string, identity TokenIdentity) error {
	if store == nil || !validMCPIdentifier(id, maxTokenIdentityLength, true) || validateTokenIdentity(identity) != nil {
		return ErrInvalidToken
	}
	record, ok, loadErr := store.loadRecord(ctx, id)
	if loadErr != nil {
		return ErrTokenPersistence
	}
	if !ok || record.workspaceID != identity.WorkspaceID || record.userID != identity.UserID {
		return ErrInvalidToken
	}
	if store.persistence != nil {
		if err := store.persistence.Revoke(ctx, id, store.now()); err != nil {
			return ErrTokenPersistence
		}
	}
	store.mu.Lock()
	record.revoked = true
	store.tokens[id] = record
	store.mu.Unlock()
	return nil
}

func (store *TokenStore) loadRecord(ctx context.Context, id string) (tokenRecord, bool, error) {
	if store.persistence == nil {
		store.mu.RLock()
		record, ok := store.tokens[id]
		store.mu.RUnlock()
		return record, ok, nil
	}
	persisted, found, err := store.persistence.Get(ctx, id)
	if err != nil || !found {
		return tokenRecord{}, found, err
	}
	record, err := tokenRecordFromPersisted(persisted)
	if err != nil {
		return tokenRecord{}, false, err
	}
	store.mu.Lock()
	store.tokens[id] = record
	store.mu.Unlock()
	return record, true, nil
}

func persistedToken(id string, record tokenRecord) PersistedToken {
	return PersistedToken{
		ID:          id,
		Digest:      append([]byte(nil), record.digest[:]...),
		Scopes:      append([]Scope(nil), record.scopes...),
		ExpiresAt:   record.expiresAt,
		Revoked:     record.revoked,
		WorkspaceID: record.workspaceID,
		UserID:      record.userID,
	}
}

func tokenRecordFromPersisted(token PersistedToken) (tokenRecord, error) {
	if len(token.Digest) != sha256.Size || token.ExpiresAt.IsZero() || (token.WorkspaceID == "") != (token.UserID == "") {
		return tokenRecord{}, ErrTokenPersistence
	}
	scopes, err := ParseScopeList(scopeStrings(token.Scopes))
	if err != nil {
		return tokenRecord{}, ErrTokenPersistence
	}
	var digest [sha256.Size]byte
	copy(digest[:], token.Digest)
	return tokenRecord{digest: digest, scopes: scopes, expiresAt: token.ExpiresAt, revoked: token.Revoked, workspaceID: token.WorkspaceID, userID: token.UserID}, nil
}

func (store *TokenStore) now() time.Time {
	if store.clock == nil {
		return time.Now().UTC()
	}
	return store.clock().UTC()
}

func randomBytes(random io.Reader, length int) ([]byte, error) {
	result := make([]byte, length)
	if _, err := io.ReadFull(random, result); err != nil {
		return nil, err
	}
	return result, nil
}

func splitToken(token string) (string, string, error) {
	if !strings.HasPrefix(token, tokenPrefix) {
		return "", "", ErrInvalidToken
	}
	value := strings.TrimPrefix(token, tokenPrefix)
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", "", ErrInvalidToken
	}
	if _, err := base64.RawURLEncoding.DecodeString(parts[0]); err != nil {
		return "", "", ErrInvalidToken
	}
	if _, err := base64.RawURLEncoding.DecodeString(parts[1]); err != nil {
		return "", "", ErrInvalidToken
	}
	return parts[0], parts[1], nil
}

func scopeStrings(scopes []Scope) []string {
	result := make([]string, len(scopes))
	for i, scope := range scopes {
		result[i] = string(scope)
	}
	return result
}
