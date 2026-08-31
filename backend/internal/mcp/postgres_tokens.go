package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTokenPersistence stores only MCP token metadata and a one-way
// digest. It is the source of truth when the process-local TokenStore is
// configured for production.
type PostgresTokenPersistence struct {
	Pool *pgxpool.Pool
}

func NewPostgresTokenPersistence(pool *pgxpool.Pool) (*PostgresTokenPersistence, error) {
	if pool == nil {
		return nil, errors.New("MCP token pool is required")
	}
	return &PostgresTokenPersistence{Pool: pool}, nil
}

func (p *PostgresTokenPersistence) Put(ctx context.Context, token PersistedToken) error {
	if p == nil || p.Pool == nil || !validMCPIdentifier(token.ID, maxTokenIdentityLength, true) || len(token.Digest) != tokenSecretDigestBytes || token.ExpiresAt.IsZero() || (token.WorkspaceID == "") != (token.UserID == "") {
		return ErrTokenPersistence
	}
	scopes, err := ParseScopeList(scopeStrings(token.Scopes))
	if err != nil {
		return ErrTokenPersistence
	}
	encodedScopes, err := json.Marshal(scopeStrings(scopes))
	if err != nil {
		return ErrTokenPersistence
	}
	result, err := p.Pool.Exec(ctx, `
		INSERT INTO mcp_tokens
			(id, digest, scopes, workspace_id, user_id, expires_at, revoked_at, created_at, updated_at)
		VALUES ($1,$2,$3::jsonb,NULLIF($4,''),NULLIF($5,''),$6,NULL,$7,$7)
		ON CONFLICT (id) DO NOTHING
	`, token.ID, append([]byte(nil), token.Digest...), encodedScopes, token.WorkspaceID, token.UserID, token.ExpiresAt.UTC(), time.Now().UTC())
	if err != nil {
		return ErrTokenPersistence
	}
	if result.RowsAffected() == 1 {
		return nil
	}
	previous, found, err := p.Get(ctx, token.ID)
	if err != nil || !found || !samePersistedToken(previous, token, scopes) {
		return ErrTokenPersistence
	}
	return nil
}

func (p *PostgresTokenPersistence) Get(ctx context.Context, id string) (PersistedToken, bool, error) {
	if p == nil || p.Pool == nil || !validMCPIdentifier(id, maxTokenIdentityLength, true) {
		return PersistedToken{}, false, ErrTokenPersistence
	}
	var token PersistedToken
	var rawScopes []byte
	var workspaceID, userID *string
	var revokedAt *time.Time
	err := p.Pool.QueryRow(ctx, `
		SELECT id, digest, scopes, workspace_id, user_id, expires_at, revoked_at
		FROM mcp_tokens WHERE id=$1
	`, id).Scan(&token.ID, &token.Digest, &rawScopes, &workspaceID, &userID, &token.ExpiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PersistedToken{}, false, nil
	}
	if err != nil {
		return PersistedToken{}, false, ErrTokenPersistence
	}
	var rawScopeValues []string
	if err := json.Unmarshal(rawScopes, &rawScopeValues); err != nil {
		return PersistedToken{}, false, ErrTokenPersistence
	}
	scopes, err := ParseScopes(strings.Join(rawScopeValues, " "))
	if err != nil {
		return PersistedToken{}, false, ErrTokenPersistence
	}
	if workspaceID != nil {
		token.WorkspaceID = *workspaceID
	}
	if userID != nil {
		token.UserID = *userID
	}
	if (token.WorkspaceID == "") != (token.UserID == "") || len(token.Digest) != tokenSecretDigestBytes {
		return PersistedToken{}, false, ErrTokenPersistence
	}
	token.Scopes = scopes
	token.Revoked = revokedAt != nil
	return token, true, nil
}

func (p *PostgresTokenPersistence) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	if p == nil || p.Pool == nil || !validMCPIdentifier(id, maxTokenIdentityLength, true) {
		return ErrTokenPersistence
	}
	if revokedAt.IsZero() {
		revokedAt = time.Now().UTC()
	}
	result, err := p.Pool.Exec(ctx, `
		UPDATE mcp_tokens
		SET revoked_at=COALESCE(revoked_at,$2), updated_at=$2
		WHERE id=$1
	`, id, revokedAt.UTC())
	if err != nil || result.RowsAffected() != 1 {
		return ErrTokenPersistence
	}
	return nil
}

const tokenSecretDigestBytes = 32

func samePersistedToken(left, right PersistedToken, normalizedScopes []Scope) bool {
	if subtle.ConstantTimeCompare(left.Digest, right.Digest) != 1 || !left.ExpiresAt.Equal(right.ExpiresAt) || left.Revoked != right.Revoked || left.WorkspaceID != right.WorkspaceID || left.UserID != right.UserID {
		return false
	}
	return stringScopesEqual(left.Scopes, normalizedScopes)
}

func stringScopesEqual(left, right []Scope) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
