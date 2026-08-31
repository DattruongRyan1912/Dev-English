package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/actions"
	productapp "github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func TestApplicationMutationErrorsMapStableRPCBoundaries(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "work validation", err: &work.ValidationError{Field: "name", Message: "required"}, code: InvalidParams},
		{name: "knowledge validation", err: &knowledge.ValidationError{Field: "content", Reason: "required"}, code: InvalidParams},
		{name: "knowledge invalid input", err: knowledge.ErrInvalidInput, code: InvalidParams},
		{name: "idempotency", err: work.ErrIdempotencyConflict, code: IdempotencyError},
		{name: "version", err: work.ErrVersionConflict, code: ConflictError},
		{name: "dependency", err: work.ErrDependenciesExist, code: ConflictError},
		{name: "knowledge conflict", err: knowledge.ErrConflict, code: ConflictError},
		{name: "work not found", err: work.ErrNotFound, code: NotFoundError},
		{name: "knowledge not found", err: knowledge.ErrNotFound, code: NotFoundError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := applicationMutationError("test.mutation", test.err)
			assertApplicationRPCCode(t, err, test.code)
			if !errors.Is(err, test.err) {
				t.Fatalf("mapped error %v does not preserve cause %v", err, test.err)
			}
		})
	}

	secret := "provider-token-secret"
	err := applicationMutationError("test.mutation", errors.New(secret))
	if !errors.Is(err, ErrApplicationDependency) || strings.Contains(err.Error(), secret) {
		t.Fatalf("generic mutation error = %v, want redacted dependency", err)
	}
}

func TestApplicationActionErrorsMapChallengeAndProviderBoundaries(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "invalid scope", err: actions.ErrInvalidScope, code: InvalidParams},
		{name: "invalid request", err: actions.ErrInvalidRequest, code: InvalidParams},
		{name: "unsupported action", err: actions.ErrUnsupportedAction, code: InvalidParams},
		{name: "invalid target", err: connectors.ErrInvalidWriteTarget, code: InvalidParams},
		{name: "invalid repository", err: connectors.ErrInvalidRepository, code: InvalidParams},
		{name: "missing challenge", err: actions.ErrChallengeNotFound, code: NotFoundError},
		{name: "invalid challenge", err: connectors.ErrInvalidChallenge, code: NotFoundError},
		{name: "mismatch", err: actions.ErrChallengeMismatch, code: ForbiddenError},
		{name: "invalid state", err: actions.ErrInvalidActionState, code: ForbiddenError},
		{name: "expired", err: connectors.ErrChallengeExpired, code: ForbiddenError},
		{name: "used", err: connectors.ErrChallengeUsed, code: ForbiddenError},
		{name: "reused key", err: connectors.ErrIdempotencyConflict, code: ForbiddenError},
		{name: "uncertain", err: connectors.ErrReceiptUncertain, code: ConflictError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := applicationActionError("github.confirm", test.err)
			assertApplicationRPCCode(t, err, test.code)
			if !errors.Is(err, test.err) {
				t.Fatalf("mapped action error %v does not preserve cause %v", err, test.err)
			}
		})
	}

	secret := "authorization: Bearer provider-token-secret"
	err := applicationActionError("github.confirm", errors.New(secret))
	if !errors.Is(err, ErrApplicationDependency) || strings.Contains(err.Error(), secret) {
		t.Fatalf("generic action error = %v, want redacted dependency", err)
	}
}

func TestApplicationScopeDerivesIdentityFromMCPPrincipal(t *testing.T) {
	product := newFullProductFixture(t)
	application := &Application{services: applicationDependencies{Product: product}}
	principal := fullApplicationPrincipal("principal-user", productapp.WorkspaceIDForUser("principal-user"), ScopeWorkWrite)
	callerContext := store.WithUser(context.Background(), "caller-controlled-user")

	boundContext, workScope, knowledgeScope, err := application.productScopes(callerContext, principal)
	if err != nil {
		t.Fatalf("productScopes() error = %v", err)
	}
	if store.UserID(boundContext) != principal.UserID {
		t.Fatalf("bound context user = %q, want principal user %q", store.UserID(boundContext), principal.UserID)
	}
	if workScope != (work.Scope{WorkspaceID: principal.WorkspaceID, UserID: principal.UserID}) || knowledgeScope.ID != principal.WorkspaceID {
		t.Fatalf("derived scopes = work=%#v knowledge=%#v", workScope, knowledgeScope)
	}

	forged := principal
	forged.WorkspaceID = "workspace-forged"
	if _, _, _, err := application.productScopes(context.Background(), forged); !errors.Is(err, ErrApplicationScopeMismatch) {
		t.Fatalf("forged workspace error = %v, want ErrApplicationScopeMismatch", err)
	}
	if _, _, _, err := application.productScopes(context.Background(), Principal{WorkspaceID: principal.WorkspaceID}); !errors.Is(err, ErrInvalidApplicationIdentity) {
		t.Fatalf("unbound MCP identity error = %v, want ErrInvalidApplicationIdentity", err)
	}
}

func TestApplicationPayloadHelpersPreserveCanonicalSearchIdentity(t *testing.T) {
	var arguments applicationIDArguments
	if err := json.Unmarshal(applicationIDArgumentsJSON("claim-1"), &arguments); err != nil {
		t.Fatalf("applicationIDArgumentsJSON() decode error = %v", err)
	}
	if arguments.ID != "claim-1" {
		t.Fatalf("applicationIDArgumentsJSON() = %#v", arguments)
	}

	items := []knowledge.SearchResult{
		{
			Source:   knowledge.KnowledgeSource{ID: "source-1", Name: "Source"},
			Item:     knowledge.SourceItem{ID: "item-1", URI: " item-uri "},
			Revision: knowledge.SourceRevision{ID: "revision-1", SourceURI: " revision-uri "},
			Chunk:    knowledge.KnowledgeChunk{ID: "chunk-1", Text: "excerpt"},
		},
		{
			Source:   knowledge.KnowledgeSource{ID: "source-2", Name: "Source 2", URI: "source-uri"},
			Item:     knowledge.SourceItem{ID: "item-2"},
			Revision: knowledge.SourceRevision{ID: "revision-2"},
			Chunk:    knowledge.KnowledgeChunk{ID: "chunk-2", Text: "second"},
		},
	}
	payloads := toKnowledgeSearchPayloads(items)
	if len(payloads) != 2 || payloads[0].ID != "knowledge-chunk-1" || payloads[0].URI != "revision-uri" || payloads[1].URI != "source-uri" || payloads[0].RetrievalMode != "lexical" {
		t.Fatalf("search payloads = %#v", payloads)
	}
	if got := firstNonEmptyApplication(" ", " trimmed ", "fallback"); got != "trimmed" {
		t.Fatalf("firstNonEmptyApplication() = %q", got)
	}
	if got := normalizeApplicationLimit(0); got != applicationMaxListItems {
		t.Fatalf("normalizeApplicationLimit(0) = %d", got)
	}
	if got := normalizeApplicationLimit(applicationMaxListItems + 1); got != applicationMaxListItems {
		t.Fatalf("normalizeApplicationLimit(too large) = %d", got)
	}
}

func TestPersistedTokenComparisonRequiresEveryIdentityField(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	left := PersistedToken{
		ID: "token-1", Digest: bytesForTokenTest(1), Scopes: []Scope{ScopeKnowledgeRead}, ExpiresAt: now,
		WorkspaceID: "workspace-1", UserID: "user-1",
	}
	right := left
	if !samePersistedToken(left, right, []Scope{ScopeKnowledgeRead}) || !stringScopesEqual(left.Scopes, right.Scopes) {
		t.Fatal("identical persisted tokens should compare equal")
	}
	mutations := []func(*PersistedToken){
		func(token *PersistedToken) { token.Digest[0]++ },
		func(token *PersistedToken) { token.ExpiresAt = now.Add(time.Minute) },
		func(token *PersistedToken) { token.Revoked = true },
		func(token *PersistedToken) { token.WorkspaceID = "workspace-2" },
		func(token *PersistedToken) { token.UserID = "user-2" },
	}
	for index, mutate := range mutations {
		candidate := left
		candidate.Digest = append([]byte(nil), left.Digest...)
		mutate(&candidate)
		if samePersistedToken(left, candidate, []Scope{ScopeKnowledgeRead}) {
			t.Fatalf("mutation %d was treated as equal", index)
		}
	}
	if stringScopesEqual([]Scope{ScopeKnowledgeRead}, []Scope{ScopeWorkRead}) || stringScopesEqual([]Scope{ScopeKnowledgeRead}, nil) {
		t.Fatal("different scope lists were treated as equal")
	}
}

func TestApplicationReadOnlyFallbackAndEncodingBoundaries(t *testing.T) {
	application, _, _, _ := newApplicationFixture(t)
	principal := applicationPrincipal()
	ctx := context.Background()

	// The compatibility application intentionally exposes only claim/chunk
	// reads when it has no canonical Product composition. The stable search
	// adapter must fail closed instead of silently inventing a repository path.
	if _, err := application.handleKnowledgeSearch(ctx, Invocation{
		Principal: principal,
		Arguments: json.RawMessage(`{"query":"canonical"}`),
	}); !errors.Is(err, ErrApplicationDependency) || !errors.Is(err, knowledge.ErrUnsupportedRead) {
		t.Fatalf("fallback knowledge search error = %v, want redacted unsupported-read dependency", err)
	}

	if _, err := application.handleKnowledgeGet(ctx, Invocation{
		Principal: principal,
		Arguments: json.RawMessage(`{"kind":"source","id":"source-1"}`),
	}); !errors.Is(err, ErrApplicationDependency) || !errors.Is(err, knowledge.ErrUnsupportedRead) {
		t.Fatalf("fallback knowledge source error = %v, want redacted unsupported-read dependency", err)
	}

	claimResult, err := application.handleKnowledgeGet(ctx, Invocation{
		Principal: principal,
		Arguments: json.RawMessage(`{"kind":"claim","id":"claim-1"}`),
	})
	if err != nil || claimResult.StructuredContent == nil {
		t.Fatalf("fallback knowledge claim result = %#v, error = %v", claimResult, err)
	}
	chunkResult, err := application.handleKnowledgeGet(ctx, Invocation{
		Principal: principal,
		Arguments: json.RawMessage(`{"kind":"chunk","id":"chunk-1"}`),
	})
	if err != nil || chunkResult.StructuredContent == nil {
		t.Fatalf("fallback knowledge chunk result = %#v, error = %v", chunkResult, err)
	}

	if _, err := application.handleKnowledgeGet(ctx, Invocation{
		Principal: principal,
		Arguments: json.RawMessage(`{"kind":"invalid","id":"claim-1"}`),
	}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("invalid knowledge kind error = %v, want ErrInvalidParams", err)
	}
	if _, err := application.handleKnowledgeSearch(ctx, Invocation{
		Principal: Principal{WorkspaceID: "workspace-a", UserID: "user-a"},
		Arguments: json.RawMessage(`{"query":" canonical "}`),
	}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("trimmed knowledge query error = %v, want ErrInvalidParams", err)
	}

	if _, err := applicationJSONToolResult(make(chan int)); !errors.Is(err, ErrApplicationEncoding) {
		t.Fatalf("tool encoding error = %v, want ErrApplicationEncoding", err)
	}
	if _, err := applicationJSONResource(ResourceWorkProjectsURI, make(chan int)); !errors.Is(err, ErrApplicationEncoding) {
		t.Fatalf("resource encoding error = %v, want ErrApplicationEncoding", err)
	}
	if err := safeApplicationDependency("cancelled", context.Canceled); !errors.Is(err, context.Canceled) || errors.Is(err, ErrApplicationDependency) {
		t.Fatalf("cancelled dependency error = %v, want original context cancellation", err)
	}
}

func TestApplicationMutationBoundariesRejectMissingIdentityAndUnsafeKeys(t *testing.T) {
	product := newFullProductFixture(t)
	application, err := NewApplication(ApplicationServices{
		Assistant: product.Assistant,
		Knowledge: product.Knowledge,
		Work:      product.Work,
		Product:   product,
	})
	if err != nil {
		t.Fatal(err)
	}
	principal := fullApplicationPrincipal("mutation-user", productapp.WorkspaceIDForUser("mutation-user"), ScopeAssistantUse, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite)
	ctx := context.Background()

	cases := []struct {
		name      string
		arguments string
		meta      RequestMeta
	}{
		{name: "project_create missing idempotency", arguments: `{"name":"project"}`},
		{name: "project_create unknown field", arguments: `{"name":"project","unexpected":true}`, meta: RequestMeta{IdempotencyKey: "project-invalid"}},
		{name: "task_upsert missing title", arguments: `{}`, meta: RequestMeta{IdempotencyKey: "task-invalid"}},
		{name: "task_upsert update missing expected version", arguments: `{"id":"task-1","title":"updated"}`, meta: RequestMeta{IdempotencyKey: "task-update-invalid"}},
		{name: "decision_record missing outcome", arguments: `{"title":"decision"}`, meta: RequestMeta{IdempotencyKey: "decision-invalid"}},
		{name: "knowledge_import missing content", arguments: `{"name":"notes"}`, meta: RequestMeta{IdempotencyKey: "knowledge-invalid"}},
		{name: "entity_trash missing expected version", arguments: `{"entityType":"project","id":"project-1"}`, meta: RequestMeta{IdempotencyKey: "trash-invalid"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			toolName := map[string]string{
				"project_create missing idempotency":          "project_create",
				"project_create unknown field":                "project_create",
				"task_upsert missing title":                   "task_upsert",
				"task_upsert update missing expected version": "task_upsert",
				"decision_record missing outcome":             "decision_record",
				"knowledge_import missing content":            "knowledge_import_manual",
				"entity_trash missing expected version":       "entity_trash",
			}[test.name]
			registry := NewRegistry()
			if err := application.Register(registry); err != nil {
				t.Fatal(err)
			}
			response := NewHandler(registry, nil).Dispatch(ctx, principal, testFullApplicationCall(`"boundary"`, toolName, test.arguments), test.meta)
			if response == nil || response.Error == nil || response.Error.Code != InvalidParams {
				t.Fatalf("%s response = %#v, want InvalidParams", test.name, response)
			}
		})
	}

	if _, err := application.handleGitHubIssueCreate(ctx, Invocation{
		Principal: principal,
		Arguments: json.RawMessage(`{"challengeId":"challenge","repository":"owner/repo","title":"Issue"}`),
	}); !errors.Is(err, ErrApplicationDependency) {
		t.Fatalf("unconfigured GitHub confirmation error = %v, want dependency error", err)
	}
}

func TestTokenStoreRevokeByIDRevokesWithoutBearerSecret(t *testing.T) {
	tokenStore := NewTokenStore(WithTokenRandom(&incrementingReader{}))
	issued, err := tokenStore.IssueForIdentity([]Scope{ScopeAssistantUse}, TokenIdentity{WorkspaceID: "workspace-a", UserID: "user-a"})
	if err != nil {
		t.Fatal(err)
	}
	token := mustReveal(t, &issued)
	if err := tokenStore.RevokeByID(issued.ID); err != nil {
		t.Fatalf("RevokeByID() error = %v", err)
	}
	if _, err := tokenStore.Verify(token, time.Now().UTC()); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("Verify() after RevokeByID() = %v, want ErrTokenRevoked", err)
	}

	for _, id := range []string{"", "token id", strings.Repeat("x", maxTokenIdentityLength+1), "missing-token"} {
		if err := tokenStore.RevokeByID(id); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("RevokeByID(%q) error = %v, want ErrInvalidToken", id, err)
		}
	}
	var nilStore *TokenStore
	if err := nilStore.RevokeByID("token-1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("nil RevokeByID() error = %v, want ErrInvalidToken", err)
	}
}

func assertApplicationRPCCode(t *testing.T, err error, want int) {
	t.Helper()
	rpcError, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("error type = %T (%v), want *RPCError", err, err)
	}
	if rpcError.Code != want {
		t.Fatalf("RPC code = %d, want %d", rpcError.Code, want)
	}
}

func bytesForTokenTest(seed byte) []byte {
	digest := make([]byte, tokenSecretDigestBytes)
	for index := range digest {
		digest[index] = seed + byte(index)
	}
	return digest
}
