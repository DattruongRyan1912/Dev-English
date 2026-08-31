package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/testkit"
)

func TestPostgresTokenPersistenceSurvivesRestartAndSharesRevocation(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "mcp-token-user-" + suffix
	workspaceID := "mcp-token-workspace-" + suffix
	if _, err := fixture.Pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, $1)`, userID); err != nil {
		t.Fatalf("seed MCP token user: %v", err)
	}
	if _, err := fixture.Pool.Exec(ctx, `
		INSERT INTO workspaces (id, owner_user_id, name, slug)
		VALUES ($1, $2, $3, $4)
	`, workspaceID, userID, "MCP token integration", "mcp-token-"+suffix); err != nil {
		t.Fatalf("seed MCP token workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	persistence, err := NewPostgresTokenPersistence(fixture.Pool)
	if err != nil {
		t.Fatalf("create PostgreSQL MCP token persistence: %v", err)
	}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	identity := TokenIdentity{WorkspaceID: workspaceID, UserID: userID}
	firstStore := NewTokenStore(
		WithTokenClock(func() time.Time { return now }),
		WithTokenTTL(2*time.Hour),
		WithTokenRandom(&incrementingReader{}),
		WithTokenPersistence(persistence),
	)
	issued, err := firstStore.IssueForIdentity([]Scope{ScopeAssistantUse, ScopeKnowledgeRead}, identity)
	if err != nil {
		t.Fatalf("issue persistent MCP token: %v", err)
	}
	token := mustReveal(t, &issued)

	var digest []byte
	if err := fixture.Pool.QueryRow(ctx, `SELECT digest FROM mcp_tokens WHERE id=$1`, issued.ID).Scan(&digest); err != nil {
		t.Fatalf("read persisted MCP digest: %v", err)
	}
	if len(digest) != sha256.Size || bytes.Contains(digest, []byte(token)) {
		t.Fatalf("MCP persistence stored an invalid or clear bearer value")
	}

	secondStore := NewTokenStore(
		WithTokenClock(func() time.Time { return now }),
		WithTokenPersistence(persistence),
	)
	principal, err := secondStore.Verify(token, now)
	if err != nil {
		t.Fatalf("verify token from a second store: %v", err)
	}
	if principal.WorkspaceID != workspaceID || principal.UserID != userID || len(principal.Scopes) != 2 {
		t.Fatalf("reloaded MCP principal = %+v", principal)
	}

	wrongIdentity := TokenIdentity{WorkspaceID: workspaceID, UserID: userID + "-other"}
	if err := secondStore.RevokeByIDForIdentity(issued.ID, wrongIdentity); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("cross-identity revoke error = %v, want ErrInvalidToken", err)
	}
	if _, err := secondStore.Verify(token, now); err != nil {
		t.Fatalf("token was changed by a rejected cross-identity revoke: %v", err)
	}

	if err := secondStore.RevokeByIDForIdentity(issued.ID, identity); err != nil {
		t.Fatalf("persistent revoke: %v", err)
	}
	if _, err := firstStore.Verify(token, now); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("cross-store verify after revoke = %v, want ErrTokenRevoked", err)
	}
	persisted, found, err := persistence.Get(ctx, issued.ID)
	if err != nil || !found || !persisted.Revoked {
		t.Fatalf("persisted revoked token = %+v, found=%t, err=%v", persisted, found, err)
	}
}

func TestPostgresTokenPersistenceRejectsInvalidReplayAndCorruptMetadata(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	validID := "mcp-token-boundary-" + suffix
	corruptID := "mcp-token-corrupt-" + suffix
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM mcp_tokens WHERE id IN ($1,$2)`, validID, corruptID)
	})

	persistence, err := NewPostgresTokenPersistence(fixture.Pool)
	if err != nil {
		t.Fatalf("create PostgreSQL MCP token persistence: %v", err)
	}
	if _, err := NewPostgresTokenPersistence(nil); err == nil {
		t.Fatal("nil PostgreSQL MCP token persistence should be rejected")
	}

	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	valid := PersistedToken{
		ID: validID, Digest: bytesForTokenTest(7), Scopes: []Scope{ScopeWorkRead, ScopeKnowledgeRead},
		ExpiresAt: now,
	}
	invalid := []struct {
		name  string
		token PersistedToken
	}{
		{name: "blank id", token: PersistedToken{Digest: bytesForTokenTest(1), ExpiresAt: now}},
		{name: "trimmed id", token: PersistedToken{ID: " " + validID, Digest: bytesForTokenTest(1), ExpiresAt: now}},
		{name: "short digest", token: PersistedToken{ID: validID + "-short", Digest: []byte{1}, ExpiresAt: now}},
		{name: "missing expiry", token: PersistedToken{ID: validID + "-expiry", Digest: bytesForTokenTest(1)}},
		{name: "half identity", token: PersistedToken{ID: validID + "-identity", Digest: bytesForTokenTest(1), ExpiresAt: now, WorkspaceID: "workspace-only"}},
		{name: "unknown scope", token: PersistedToken{ID: validID + "-scope", Digest: bytesForTokenTest(1), ExpiresAt: now, Scopes: []Scope{"scope:unknown"}}},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			if err := persistence.Put(ctx, testCase.token); !errors.Is(err, ErrTokenPersistence) {
				t.Fatalf("Put() error = %v, want ErrTokenPersistence", err)
			}
		})
	}

	if err := persistence.Put(ctx, valid); err != nil {
		t.Fatalf("Put(valid) error = %v", err)
	}
	if err := persistence.Put(ctx, valid); err != nil {
		t.Fatalf("idempotent Put(valid) error = %v", err)
	}
	changed := valid
	changed.Digest = bytesForTokenTest(8)
	if err := persistence.Put(ctx, changed); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("conflicting Put() error = %v, want ErrTokenPersistence", err)
	}

	loaded, found, err := persistence.Get(ctx, validID)
	normalizedScopes := []Scope{ScopeKnowledgeRead, ScopeWorkRead}
	if err != nil || !found || !samePersistedToken(loaded, valid, normalizedScopes) {
		t.Fatalf("Get(valid) = %+v, found=%t, err=%v", loaded, found, err)
	}
	if _, found, err := persistence.Get(ctx, "missing-token-"+suffix); err != nil || found {
		t.Fatalf("Get(missing) = found=%t, err=%v, want not found", found, err)
	}
	if _, _, err := persistence.Get(ctx, " "); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("Get(invalid id) error = %v, want ErrTokenPersistence", err)
	}
	var nilPersistence *PostgresTokenPersistence
	if err := nilPersistence.Put(ctx, valid); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("nil Put() error = %v, want ErrTokenPersistence", err)
	}
	if _, _, err := nilPersistence.Get(ctx, validID); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("nil Get() error = %v, want ErrTokenPersistence", err)
	}
	if err := nilPersistence.Revoke(ctx, validID, now); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("nil Revoke() error = %v, want ErrTokenPersistence", err)
	}

	if err := persistence.Revoke(ctx, "missing-token-"+suffix, now); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("Revoke(missing) error = %v, want ErrTokenPersistence", err)
	}
	if err := persistence.Revoke(ctx, validID, time.Time{}); err != nil {
		t.Fatalf("Revoke(valid, zero time) error = %v", err)
	}
	loaded, found, err = persistence.Get(ctx, validID)
	if err != nil || !found || !loaded.Revoked {
		t.Fatalf("Get(revoked) = %+v, found=%t, err=%v", loaded, found, err)
	}
	if err := persistence.Revoke(ctx, validID, now.Add(time.Minute)); err != nil {
		t.Fatalf("idempotent Revoke(valid) error = %v", err)
	}

	if _, err := fixture.Pool.Exec(ctx, `
		INSERT INTO mcp_tokens (id, digest, scopes, expires_at)
		VALUES ($1, $2, $3::jsonb, $4)
	`, corruptID, bytesForTokenTest(9), `["not-a-real-scope"]`, now); err != nil {
		t.Fatalf("seed corrupt scope metadata: %v", err)
	}
	if _, _, err := persistence.Get(ctx, corruptID); !errors.Is(err, ErrTokenPersistence) {
		t.Fatalf("Get(corrupt scope) error = %v, want ErrTokenPersistence", err)
	}
}
