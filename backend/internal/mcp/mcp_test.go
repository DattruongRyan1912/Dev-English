package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type incrementingReader struct {
	next byte
}

func (reader *incrementingReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = reader.next
		reader.next++
	}
	return len(value), nil
}

type constantReader struct {
	value byte
}

func (reader constantReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = reader.value
	}
	return len(value), nil
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

type tokenPersistenceFixture struct {
	mu     sync.Mutex
	tokens map[string]PersistedToken
}

func (p *tokenPersistenceFixture) Put(_ context.Context, token PersistedToken) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tokens == nil {
		p.tokens = make(map[string]PersistedToken)
	}
	if previous, exists := p.tokens[token.ID]; exists {
		if previous.ID != token.ID || previous.ExpiresAt != token.ExpiresAt || previous.WorkspaceID != token.WorkspaceID || previous.UserID != token.UserID || !reflect.DeepEqual(previous.Scopes, token.Scopes) || !reflect.DeepEqual(previous.Digest, token.Digest) {
			return ErrTokenPersistence
		}
		return nil
	}
	copyToken := token
	copyToken.Digest = append([]byte(nil), token.Digest...)
	copyToken.Scopes = append([]Scope(nil), token.Scopes...)
	p.tokens[token.ID] = copyToken
	return nil
}

func (p *tokenPersistenceFixture) Get(_ context.Context, id string) (PersistedToken, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	token, ok := p.tokens[id]
	if !ok {
		return PersistedToken{}, false, nil
	}
	token.Digest = append([]byte(nil), token.Digest...)
	token.Scopes = append([]Scope(nil), token.Scopes...)
	return token, true, nil
}

func (p *tokenPersistenceFixture) Revoke(_ context.Context, id string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	token, ok := p.tokens[id]
	if !ok {
		return ErrTokenPersistence
	}
	token.Revoked = true
	p.tokens[id] = token
	return nil
}

func testRequest(id, method, params string) Request {
	request := Request{
		JSONRPC: "2.0",
		Method:  method,
	}
	if id != "" {
		request.ID = json.RawMessage(id)
	}
	if params != "" {
		request.Params = json.RawMessage(params)
	}
	return request
}

func setMCPRequestHeaders(request *http.Request, token string) {
	request.Header.Set(AuthorizationHeader, "Bearer "+token)
	request.Header.Set(AcceptHeader, "application/json, text/event-stream")
}

func mustReveal(t *testing.T, issued *IssuedToken) string {
	t.Helper()
	token, err := issued.Reveal()
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}
	if token == "" {
		t.Fatal("Reveal() returned an empty token")
	}
	return token
}

func mustResponse(t *testing.T, response *Response) *Response {
	t.Helper()
	if response == nil {
		t.Fatal("expected a JSON-RPC response")
	}
	return response
}

func responseError(t *testing.T, response *Response) *RPCError {
	t.Helper()
	response = mustResponse(t, response)
	if response.Error == nil {
		t.Fatal("expected a JSON-RPC error")
	}
	return response.Error
}

func responseResult[T any](t *testing.T, response *Response) T {
	t.Helper()
	response = mustResponse(t, response)
	if response.Error != nil {
		t.Fatalf("unexpected JSON-RPC error code %d", response.Error.Code)
	}
	var result T
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return result
}

func expectErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want errors.Is(..., %v)", err, target)
	}
}

func noopTool(context.Context, Invocation) (CallToolResult, error) {
	return CallToolResult{Content: []Content{{Type: "text", Text: "ok"}}}, nil
}

func noopResource(context.Context, ResourceRequest) ([]ResourceContent, error) {
	text := "resource"
	return []ResourceContent{{Text: &text}}, nil
}

func TestTokenStoreIssueVerifyTTLRevokeAndDigestStorage(t *testing.T) {
	current := time.Date(2026, 8, 26, 12, 0, 0, 0, time.FixedZone("test", 7*60*60))
	store := NewTokenStore(
		WithTokenClock(func() time.Time { return current }),
		WithTokenTTL(time.Hour),
		WithTokenRandom(&incrementingReader{}),
		nil,
	)

	issued, err := store.Issue([]Scope{ScopeWorkRead, ScopeKnowledgeRead, ScopeWorkRead})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !issued.ExpiresAt.Equal(current.UTC().Add(time.Hour)) {
		t.Fatalf("ExpiresAt does not use the configured clock and TTL")
	}
	if !reflect.DeepEqual(issued.Scopes, []Scope{ScopeKnowledgeRead, ScopeWorkRead}) {
		t.Fatalf("Scopes = %#v, want normalized scopes", issued.Scopes)
	}

	copyIssued := issued
	token := mustReveal(t, &issued)
	if _, err := copyIssued.Reveal(); !errors.Is(err, ErrTokenAlreadyRevealed) {
		t.Fatalf("copy Reveal() error = %v, want ErrTokenAlreadyRevealed", err)
	}

	principal, err := store.Verify(token, current)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if principal.TokenID != issued.ID || !reflect.DeepEqual(principal.Scopes, issued.Scopes) {
		t.Fatal("Verify() returned different token metadata")
	}
	if !principal.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Fatal("Verify() returned a different expiry")
	}

	_, secret, err := splitToken(token)
	if err != nil {
		t.Fatalf("splitToken() error = %v", err)
	}
	wantDigest := sha256.Sum256([]byte(secret))
	store.mu.RLock()
	record, ok := store.tokens[issued.ID]
	store.mu.RUnlock()
	if !ok || record.digest != wantDigest || len(record.scopes) != 2 {
		t.Fatal("token store did not retain the expected digest-only record")
	}

	if _, err := store.Verify(token, issued.ExpiresAt); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired Verify() error = %v, want ErrTokenExpired", err)
	}

	secondIssued, err := store.IssueString("work:write")
	if err != nil {
		t.Fatalf("IssueString() error = %v", err)
	}
	secondToken := mustReveal(t, &secondIssued)
	if err := store.Revoke(secondToken); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := store.Verify(secondToken, current); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("revoked Verify() error = %v, want ErrTokenRevoked", err)
	}
	if err := store.Revoke(secondToken); err != nil {
		t.Fatalf("repeated Revoke() error = %v", err)
	}
	if err := store.Revoke("not-a-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("invalid Revoke() error = %v, want ErrInvalidToken", err)
	}
}

func TestTokenStorePersistenceSurvivesRestartAndSharesRevocation(t *testing.T) {
	current := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	persistence := &tokenPersistenceFixture{}
	identity := TokenIdentity{WorkspaceID: "workspace-a", UserID: "user-a"}
	firstStore := NewTokenStore(
		WithTokenClock(func() time.Time { return current }),
		WithTokenRandom(&incrementingReader{}),
		WithTokenPersistence(persistence),
	)
	issued, err := firstStore.IssueForIdentity([]Scope{ScopeAssistantUse, ScopeKnowledgeRead}, identity)
	if err != nil {
		t.Fatal(err)
	}
	token := mustReveal(t, &issued)

	secondStore := NewTokenStore(
		WithTokenClock(func() time.Time { return current }),
		WithTokenPersistence(persistence),
	)
	principal, err := secondStore.Verify(token, current)
	if err != nil {
		t.Fatalf("restart Verify() error = %v", err)
	}
	if principal.WorkspaceID != identity.WorkspaceID || principal.UserID != identity.UserID || !reflect.DeepEqual(principal.Scopes, issued.Scopes) {
		t.Fatalf("restart principal = %+v", principal)
	}
	if err := secondStore.RevokeByIDForIdentity(issued.ID, identity); err != nil {
		t.Fatalf("persistent revoke error = %v", err)
	}
	if _, err := firstStore.Verify(token, current); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("cross-store Verify() error = %v, want ErrTokenRevoked", err)
	}
}

func TestIssuedTokenRevealIsSharedAcrossCopiesAndConcurrentCalls(t *testing.T) {
	store := NewTokenStore(WithTokenRandom(&incrementingReader{}))
	issued, err := store.Issue([]Scope{ScopeAssistantUse})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	const callers = 32
	copies := make([]IssuedToken, callers)
	for index := range copies {
		copies[index] = issued
	}
	results := make(chan error, callers)
	values := make(chan string, callers)
	var wait sync.WaitGroup
	for index := range copies {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			value, revealErr := copies[index].Reveal()
			values <- value
			results <- revealErr
		}(index)
	}
	wait.Wait()
	close(results)
	close(values)

	var successes int
	var successValue string
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrTokenAlreadyRevealed) {
			t.Fatalf("concurrent Reveal() error = %v", err)
		}
	}
	for value := range values {
		if value != "" {
			successValue = value
		}
	}
	if successes != 1 || successValue == "" {
		t.Fatalf("concurrent Reveal() successes = %d, want exactly one", successes)
	}

	var nilIssued *IssuedToken
	if _, err := nilIssued.Reveal(); !errors.Is(err, ErrTokenAlreadyRevealed) {
		t.Fatalf("nil Reveal() error = %v, want ErrTokenAlreadyRevealed", err)
	}
	var zeroIssued IssuedToken
	if _, err := zeroIssued.Reveal(); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("zero-value Reveal() error = %v, want ErrInvalidToken", err)
	}
}

func TestTokenStoreValidationAndFailurePaths(t *testing.T) {
	store := NewTokenStore(WithTokenClock(nil), WithTokenTTL(0), WithTokenRandom(nil), nil)
	if store.ttl != DefaultTokenTTL || store.clock == nil || store.random == nil {
		t.Fatal("nil token options should preserve defaults")
	}

	if _, err := store.Issue([]Scope{Scope("unknown:scope")}); !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("unknown Issue() scope error = %v, want ErrUnknownScope", err)
	}
	if _, err := store.IssueString("unknown:scope"); !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("unknown IssueString() scope error = %v, want ErrUnknownScope", err)
	}
	if _, err := store.Issue(nil); err != nil {
		t.Fatalf("Issue(nil) error = %v", err)
	}

	failingStore := NewTokenStore(WithTokenRandom(errorReader{}))
	if _, err := failingStore.Issue(nil); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("random source error = %v, want io.ErrUnexpectedEOF", err)
	}

	constantStore := NewTokenStore(WithTokenRandom(constantReader{value: 7}))
	first, err := constantStore.Issue(nil)
	if err != nil {
		t.Fatalf("first constant Issue() error = %v", err)
	}
	if _, err := constantStore.Issue(nil); err == nil {
		t.Fatal("repeated token ID should fail closed after allocation retries")
	}
	_ = first

	malformed := []string{
		"",
		"mcp_",
		"mcp_a",
		"mcp_a.b.c",
		"mcp_a.b c",
		"mcp_!.Yg",
		"mcp_Yg.!",
		"mcp_Yg.Yg",
	}
	for _, token := range malformed {
		if _, err := store.Verify(token, time.Time{}); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("malformed Verify() error = %v, want ErrInvalidToken", err)
		}
	}
	if _, err := store.VerifyAuthorization("Basic value", time.Time{}); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("invalid authorization error = %v, want ErrInvalidAuthorization", err)
	}
}

func TestScopesAndScopeSets(t *testing.T) {
	wantKnown := []Scope{ScopeAssistantUse, ScopeGitHubWrite, ScopeKnowledgeRead, ScopeWorkRead, ScopeWorkWrite}
	known := KnownScopes()
	if !reflect.DeepEqual(known, wantKnown) {
		t.Fatalf("KnownScopes() = %#v, want %#v", known, wantKnown)
	}
	known[0] = Scope("mutated")
	if reflect.DeepEqual(KnownScopes(), known) {
		t.Fatal("KnownScopes() returned mutable shared storage")
	}

	parsed, err := ParseScopes("work:read, knowledge:read work:read")
	if err != nil || !reflect.DeepEqual(parsed, []Scope{ScopeKnowledgeRead, ScopeWorkRead}) {
		t.Fatalf("ParseScopes() = %#v, %v", parsed, err)
	}
	if empty, err := ParseScopes(" , "); err != nil || len(empty) != 0 {
		t.Fatalf("empty ParseScopes() = %#v, %v", empty, err)
	}
	if _, err := ParseScopes("work:read unknown:scope"); !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("unknown ParseScopes() error = %v", err)
	}
	if _, err := ParseScopeList([]string{string(ScopeWorkWrite), "unknown:scope"}); !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("unknown ParseScopeList() error = %v", err)
	}

	set, err := NewScopeSet(ScopeWorkWrite, ScopeKnowledgeRead, ScopeWorkWrite)
	if err != nil {
		t.Fatalf("NewScopeSet() error = %v", err)
	}
	if !set.Has(ScopeWorkWrite) || set.Has(ScopeGitHubWrite) {
		t.Fatal("ScopeSet.Has() returned the wrong membership")
	}
	if err := set.Require(ScopeWorkWrite); err != nil {
		t.Fatalf("ScopeSet.Require() error = %v", err)
	}
	if err := set.Require(ScopeAssistantUse); !errors.Is(err, ErrMissingScope) {
		t.Fatalf("missing ScopeSet.Require() error = %v", err)
	}
	if err := set.Require(""); err != nil {
		t.Fatalf("empty ScopeSet.Require() error = %v", err)
	}
	if !reflect.DeepEqual(set.List(), []Scope{ScopeKnowledgeRead, ScopeWorkWrite}) {
		t.Fatalf("ScopeSet.List() = %#v", set.List())
	}
	if _, err := NewScopeSet(Scope("unknown:scope")); !errors.Is(err, ErrUnknownScope) {
		t.Fatalf("unknown NewScopeSet() error = %v", err)
	}

	principal := Principal{Scopes: []Scope{ScopeKnowledgeRead}}
	if !principal.HasScope(ScopeKnowledgeRead) || !principal.HasScope("") || principal.HasScope(ScopeWorkRead) {
		t.Fatal("Principal.HasScope() returned the wrong result")
	}
	if err := principal.RequireScope(ScopeKnowledgeRead); err != nil {
		t.Fatalf("Principal.RequireScope() error = %v", err)
	}
	if err := principal.RequireScope(ScopeWorkRead); !errors.Is(err, ErrMissingScope) {
		t.Fatalf("missing Principal.RequireScope() error = %v", err)
	}
	if !isWriteScope(ScopeWorkWrite) || !isWriteScope(ScopeGitHubWrite) || isWriteScope(ScopeWorkRead) || isWriteScope("") {
		t.Fatal("write scope allow-list is incorrect")
	}
}

func TestRegistryScopeValidationAndListing(t *testing.T) {
	var nilRegistry *Registry
	if err := nilRegistry.RegisterTool(Tool{}); !errors.Is(err, ErrInvalidTool) {
		t.Fatalf("nil RegisterTool() error = %v", err)
	}
	if err := nilRegistry.RegisterResource(Resource{}); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("nil RegisterResource() error = %v", err)
	}

	registry := NewRegistry()
	invalidTools := []struct {
		name string
		tool Tool
		want error
	}{
		{name: "blank", tool: Tool{Name: " ", Handler: noopTool}, want: ErrInvalidTool},
		{name: "long", tool: Tool{Name: strings.Repeat("x", 129), Handler: noopTool}, want: ErrInvalidTool},
		{name: "control", tool: Tool{Name: "bad\nname", Handler: noopTool}, want: ErrInvalidTool},
		{name: "handler", tool: Tool{Name: "missing-handler"}, want: ErrInvalidTool},
		{name: "unknown-scope", tool: Tool{Name: "unknown-scope", RequiredScope: Scope("unknown"), Handler: noopTool}, want: ErrUnknownScope},
		{name: "empty-write", tool: Tool{Name: "empty-write", Mutating: true, Handler: noopTool}, want: ErrInvalidTool},
		{name: "read-write", tool: Tool{Name: "read-write", Mutating: true, RequiredScope: ScopeWorkRead, Handler: noopTool}, want: ErrInvalidTool},
		{name: "assistant-write", tool: Tool{Name: "assistant-write", Mutating: true, RequiredScope: ScopeAssistantUse, Handler: noopTool}, want: ErrInvalidTool},
	}
	for _, testCase := range invalidTools {
		if err := registry.RegisterTool(testCase.tool); !errors.Is(err, testCase.want) {
			t.Errorf("%s RegisterTool() error = %v, want %v", testCase.name, err, testCase.want)
		}
	}

	validTools := []Tool{
		{Name: " public-read ", Handler: noopTool},
		{Name: "knowledge-read", RequiredScope: ScopeKnowledgeRead, Handler: noopTool},
		{Name: "work-write", RequiredScope: ScopeWorkWrite, Mutating: true, Handler: noopTool},
		{Name: "github-write", RequiredScope: ScopeGitHubWrite, Mutating: true, Handler: noopTool},
	}
	for _, tool := range validTools {
		if err := registry.RegisterTool(tool); err != nil {
			t.Fatalf("valid RegisterTool(%q) error = %v", tool.Name, err)
		}
	}
	if err := registry.RegisterTool(Tool{Name: "work-write", RequiredScope: ScopeWorkWrite, Mutating: true, Handler: noopTool}); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("duplicate RegisterTool() error = %v, want ErrDuplicateTool", err)
	}
	if err := registry.RegisterTool(Tool{Name: "read-write", RequiredScope: ScopeWorkWrite, Handler: noopTool}); err != nil {
		t.Fatalf("read-only write-scoped RegisterTool() error = %v", err)
	}

	tools := registry.ListTools(Principal{Scopes: []Scope{ScopeKnowledgeRead, ScopeWorkWrite}})
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	if !sort.StringsAreSorted(names) || !reflect.DeepEqual(names, []string{"knowledge-read", "public-read", "read-write", "work-write"}) {
		t.Fatalf("ListTools() names = %#v", names)
	}
	for index := range tools {
		if tools[index].Name == "public-read" && string(tools[index].InputSchema) != defaultInputSchema {
			t.Fatal("empty input schema was not normalized")
		}
	}
	tools[0].InputSchema[0] = 'x'
	if string(registry.ListTools(Principal{Scopes: []Scope{ScopeKnowledgeRead}})[0].InputSchema) == string(tools[0].InputSchema) {
		t.Fatal("ListTools() leaked mutable schema storage")
	}

	invalidResources := []struct {
		name     string
		resource Resource
		want     error
	}{
		{name: "blank", resource: Resource{URI: " "}, want: ErrInvalidResource},
		{name: "long", resource: Resource{URI: strings.Repeat("u", 2049), Reader: noopResource}, want: ErrInvalidResource},
		{name: "control", resource: Resource{URI: "urn:bad\nuri", Reader: noopResource}, want: ErrInvalidResource},
		{name: "reader", resource: Resource{URI: "urn:no-reader"}, want: ErrInvalidResource},
		{name: "unknown-scope", resource: Resource{URI: "urn:unknown", RequiredScope: Scope("unknown"), Reader: noopResource}, want: ErrUnknownScope},
	}
	for _, testCase := range invalidResources {
		if err := registry.RegisterResource(testCase.resource); !errors.Is(err, testCase.want) {
			t.Errorf("%s RegisterResource() error = %v, want %v", testCase.name, err, testCase.want)
		}
	}
	for _, resource := range []Resource{
		{URI: " urn:public ", Name: "Public", Reader: noopResource},
		{URI: "urn:knowledge", RequiredScope: ScopeKnowledgeRead, Reader: noopResource},
	} {
		if err := registry.RegisterResource(resource); err != nil {
			t.Fatalf("valid RegisterResource(%q) error = %v", resource.URI, err)
		}
	}
	if err := registry.RegisterResource(Resource{URI: "urn:public", Reader: noopResource}); !errors.Is(err, ErrDuplicateResource) {
		t.Fatalf("duplicate RegisterResource() error = %v, want ErrDuplicateResource", err)
	}
	resources := registry.ListResources(Principal{Scopes: []Scope{ScopeKnowledgeRead}})
	if len(resources) != 2 || resources[0].URI != "urn:knowledge" || resources[1].URI != "urn:public" {
		t.Fatalf("ListResources() = %#v", resources)
	}

	if _, ok := registry.tool("work-write"); !ok {
		t.Fatal("registered tool was not found")
	}
	if _, ok := registry.resource("urn:public"); !ok {
		t.Fatal("registered resource was not found")
	}
	var nilTools *Registry
	if nilTools.ListTools(Principal{}) != nil || nilTools.ListResources(Principal{}) != nil {
		t.Fatal("nil registry listing should return nil")
	}

	for _, schema := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`[]`), json.RawMessage(`"string"`), json.RawMessage(`{`)} {
		if schema == nil {
			continue
		}
		if _, err := normalizeSchema(schema); !errors.Is(err, ErrInvalidTool) {
			t.Errorf("normalizeSchema(%s) error = %v, want ErrInvalidTool", schema, err)
		}
	}
	if normalized, err := normalizeSchema(json.RawMessage(`{"type":"object"}`)); err != nil || len(normalized) == 0 {
		t.Fatalf("valid normalizeSchema() = %s, %v", normalized, err)
	}
}

func TestReplayGuardNonceIdempotencyAndCanonicalHash(t *testing.T) {
	now := time.Date(2026, 8, 26, 13, 0, 0, 0, time.UTC)
	guard := NewReplayGuard(
		WithReplayClock(func() time.Time { return now }),
		WithReplayTTL(time.Minute),
		nil,
	)
	if guard.ttl != time.Minute {
		t.Fatal("WithReplayTTL() did not configure the TTL")
	}
	if err := guard.RecordNonce("", "nonce", now); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("empty nonce subject error = %v", err)
	}
	if err := guard.RecordNonce("subject", "", now); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("empty nonce error = %v", err)
	}
	if err := guard.RecordNonce("subject", "nonce", now); err != nil {
		t.Fatalf("RecordNonce() error = %v", err)
	}
	if err := guard.RecordNonce("subject", "nonce", now); !errors.Is(err, ErrReplayDetected) {
		t.Fatalf("duplicate nonce error = %v", err)
	}
	if err := guard.RecordNonce("other-subject", "nonce", now); err != nil {
		t.Fatalf("subject-scoped nonce error = %v", err)
	}
	if err := guard.RecordNonce("subject", "expired", now); err != nil {
		t.Fatalf("second nonce error = %v", err)
	}
	now = now.Add(time.Minute)
	if err := guard.RecordNonce("subject", "nonce", now); err != nil {
		t.Fatalf("expired nonce was not pruned: %v", err)
	}

	hash := sha256.Sum256([]byte("action"))
	if cached, replay, err := guard.BeginIdempotency("subject", "", hash, now); err != nil || replay || cached != nil {
		t.Fatalf("empty idempotency key = %v, %t, %v", err, replay, cached)
	}
	if _, _, err := guard.BeginIdempotency("", "key", hash, now); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("empty idempotency subject error = %v", err)
	}
	if _, _, err := guard.BeginIdempotency("subject", "bad\nkey", hash, now); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("invalid idempotency key error = %v", err)
	}
	if cached, replay, err := guard.BeginIdempotency("subject", "key", hash, now); err != nil || replay || cached != nil {
		t.Fatalf("first idempotency reservation = %v, %t, %v", err, replay, cached)
	}
	if _, _, err := guard.BeginIdempotency("subject", "key", hash, now); !errors.Is(err, ErrIdempotencyInProgress) {
		t.Fatalf("pending idempotency error = %v", err)
	}
	otherHash := sha256.Sum256([]byte("other-action"))
	if _, _, err := guard.BeginIdempotency("subject", "key", otherHash, now); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
	if err := guard.CompleteIdempotency("subject", "key", otherHash, []byte("ignored"), now); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("CompleteIdempotency() conflict error = %v", err)
	}
	if err := guard.CompleteIdempotency("", "key", hash, nil, now); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("CompleteIdempotency() empty subject error = %v", err)
	}
	if err := guard.CompleteIdempotency("subject", "", hash, nil, now); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("CompleteIdempotency() empty key error = %v", err)
	}
	if err := guard.CompleteIdempotency("subject", "missing", hash, nil, now); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("CompleteIdempotency() missing key error = %v", err)
	}
	response := []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	if err := guard.CompleteIdempotency("subject", "key", hash, response, now); err != nil {
		t.Fatalf("CompleteIdempotency() error = %v", err)
	}
	response[0] = 'x'
	cached, replay, err := guard.BeginIdempotency("subject", "key", hash, now)
	if err != nil || !replay || !bytes.Equal(cached, []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)) {
		t.Fatalf("cached idempotency response = %s, %t, %v", cached, replay, err)
	}
	if err := guard.CompleteIdempotency("subject", "key", hash, nil, now); err != nil {
		t.Fatalf("repeated CompleteIdempotency() error = %v", err)
	}

	if cached, replay, err := guard.BeginIdempotency("subject", "expired-key", hash, now); err != nil || replay || cached != nil {
		t.Fatalf("second reservation = %v, %t, %v", err, replay, cached)
	}
	if err := guard.AbortIdempotency("subject", "expired-key", otherHash); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("AbortIdempotency() conflict error = %v", err)
	}
	if err := guard.AbortIdempotency("subject", "expired-key", hash); err != nil {
		t.Fatalf("AbortIdempotency() error = %v", err)
	}
	if err := guard.AbortIdempotency("subject", "expired-key", hash); err != nil {
		t.Fatalf("missing AbortIdempotency() error = %v", err)
	}
	if _, replay, err := guard.BeginIdempotency("subject", "expired-key", hash, now); err != nil || replay {
		t.Fatalf("aborted key was not reusable: %v, %t", err, replay)
	}
	if err := guard.AbortIdempotency("", "key", hash); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("AbortIdempotency() empty subject error = %v", err)
	}
	if err := guard.AbortIdempotency("subject", "", hash); !errors.Is(err, ErrInvalidIdempotencyKey) {
		t.Fatalf("AbortIdempotency() empty key error = %v", err)
	}

	for _, nonce := range []string{" ", "bad\nnonce", "bad\tnonce", strings.Repeat("n", maxNonceLength+1)} {
		if err := validateNonce(nonce); !errors.Is(err, ErrInvalidNonce) {
			t.Errorf("validateNonce(%q) = %v", nonce, err)
		}
	}
	for _, key := range []string{" ", "bad\nkey", "bad\tkey", strings.Repeat("k", maxIdempotencyKeySize+1)} {
		if err := validateIdempotencyKey(key); !errors.Is(err, ErrInvalidIdempotencyKey) {
			t.Errorf("validateIdempotencyKey(%q) = %v", key, err)
		}
	}

	canonical, err := CanonicalJSON([]byte(" {\"b\":2, \"a\":1} "))
	if err != nil || string(canonical) != `{"a":1,"b":2}` {
		t.Fatalf("CanonicalJSON() = %s, %v", canonical, err)
	}
	if canonical, err := CanonicalJSON(nil); err != nil || string(canonical) != "null" {
		t.Fatalf("empty CanonicalJSON() = %s, %v", canonical, err)
	}
	for _, raw := range []string{"{", "{} {}"} {
		if _, err := CanonicalJSON([]byte(raw)); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("CanonicalJSON(%q) error = %v", raw, err)
		}
	}
	if _, err := ActionHash("", json.RawMessage(`{}`)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty ActionHash() method error = %v", err)
	}
	if _, err := ActionHash("tools/call", json.RawMessage(`{`)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid ActionHash() params error = %v", err)
	}
	hashA, err := ActionHash("tools/call", json.RawMessage(`{"a":1}`))
	if err != nil {
		t.Fatalf("valid ActionHash() error = %v", err)
	}
	hashB, err := ActionHash("tools/call", json.RawMessage(`{"a":2}`))
	if err != nil || hashA == hashB {
		t.Fatalf("different ActionHash() inputs should differ: %v", err)
	}

	var nilClockGuard ReplayGuard
	if nilClockGuard.now().IsZero() {
		t.Fatal("nil ReplayGuard clock should fall back to the system clock")
	}
}

func TestProtocolDecodeAndResponseHelpers(t *testing.T) {
	request, err := DecodeRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`))
	if err != nil || request.IsNotification() || string(request.ID) != "1" {
		t.Fatalf("valid DecodeRequest() = %#v, %v", request, err)
	}
	notification, err := DecodeRequest([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil || !notification.IsNotification() {
		t.Fatalf("notification DecodeRequest() = %#v, %v", notification, err)
	}
	invalidBodies := []string{
		"{",
		`{"jsonrpc":"1.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":1,"method":""}`,
		`{"jsonrpc":"2.0","id":1,"method":"bad\nmethod"}`,
		`{"jsonrpc":"2.0","id":null,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":true,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":{},"method":"ping"}`,
		`{"jsonrpc":"2.0","id":1,"method":"ping"} {}`,
	}
	for _, body := range invalidBodies {
		if _, err := DecodeRequest([]byte(body)); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("DecodeRequest(%q) error = %v", body, err)
		}
	}
	for _, raw := range []json.RawMessage{json.RawMessage(`"id"`), json.RawMessage(`10`), json.RawMessage(`-1.5`)} {
		if !validRequestID(raw) {
			t.Errorf("validRequestID(%s) = false", raw)
		}
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`true`), json.RawMessage(`[]`), json.RawMessage(`"`)} {
		if validRequestID(raw) {
			t.Errorf("validRequestID(%s) = true", raw)
		}
	}

	id := json.RawMessage(`"original"`)
	resultResponse, err := NewResultResponse(id, map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("NewResultResponse() error = %v", err)
	}
	id[1] = 'x'
	if string(resultResponse.ID) != `"original"` {
		t.Fatal("NewResultResponse() did not clone the request ID")
	}
	if _, err := NewResultResponse(nil, func() {}); err == nil {
		t.Fatal("NewResultResponse() should reject an unmarshalable result")
	}
	errorResponse := NewErrorResponse(json.RawMessage("2"), newRPCError(InternalError, "failure", errors.New("cause"), nil))
	if string(errorResponse.ID) != "2" || errorResponse.Error == nil {
		t.Fatal("NewErrorResponse() did not retain its ID and error")
	}
	var nilRPCError *RPCError
	if nilRPCError.Error() != "" || nilRPCError.Unwrap() != nil {
		t.Fatal("nil RPCError methods should be safe")
	}
	if errorResponse.Error.Error() != "failure" || errorResponse.Error.Unwrap() == nil {
		t.Fatal("RPCError methods did not expose safe metadata")
	}
}

func TestHandlerInitializeDiscoveryAndDispatchErrors(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterTool(Tool{Name: "read", RequiredScope: ScopeKnowledgeRead, Handler: noopTool}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}
	if err := registry.RegisterResource(Resource{URI: "urn:read", RequiredScope: ScopeKnowledgeRead, Reader: noopResource}); err != nil {
		t.Fatalf("RegisterResource() error = %v", err)
	}
	replay := NewReplayGuard()
	handler := NewHandler(
		registry,
		nil,
		WithHandlerClock(func() time.Time { return time.Unix(100, 0) }),
		WithProtocolVersion(" custom "),
		WithSupportedProtocolVersions("", "legacy", "legacy"),
		WithServerInfo(Implementation{Name: " server ", Version: " version "}),
		WithServerInstructions(" guidance "),
		WithMaxBodyBytes(128),
		WithReplayGuard(replay),
		nil,
	)
	if handler.ProtocolVersion != "custom" || handler.ServerInfo.Name != "server" || handler.Instructions != "guidance" || handler.MaxBodyBytes != 128 || handler.Replay != replay || len(handler.SupportedProtocolVersions) != 1 {
		t.Fatal("handler options were not applied as expected")
	}
	defaultHandler := NewHandler(nil, nil, WithProtocolVersion(" "), WithSupportedProtocolVersions("", ""), WithServerInfo(Implementation{}), WithMaxBodyBytes(0), WithReplayGuard(nil), nil)
	if defaultHandler.Registry == nil || defaultHandler.MaxBodyBytes != DefaultMaxBodyBytes || defaultHandler.Instructions != DefaultServerInstructions {
		t.Fatal("handler defaults were not preserved")
	}
	if len([]rune(DefaultServerInstructions)) >= 512 {
		t.Fatalf("default server instructions are too long: %d runes", len([]rune(DefaultServerInstructions)))
	}

	initParams := `{"protocolVersion":"custom","capabilities":{},"clientInfo":{"name":"client","version":"1"}}`
	initResponse := mustResponse(t, handler.Dispatch(context.Background(), Principal{}, testRequest("1", "initialize", initParams), RequestMeta{}))
	initResult := responseResult[InitializeResult](t, initResponse)
	if initResult.ProtocolVersion != "custom" || initResult.ServerInfo.Name != "server" || initResult.Instructions != "guidance" {
		t.Fatal("initialize response did not negotiate the configured version")
	}
	discoveryResult := responseResult[DiscoverResult](t, handler.Dispatch(context.Background(), Principal{}, testRequest("1-discovery", "server/discover", `{}`), RequestMeta{}))
	if discoveryResult.Instructions != "guidance" {
		t.Fatalf("discovery instructions = %q, want configured guidance", discoveryResult.Instructions)
	}
	legacyResponse := handler.Dispatch(context.Background(), Principal{}, testRequest("2", "initialize", `{"protocolVersion":"legacy"}`), RequestMeta{})
	if legacyResponse == nil || legacyResponse.Error != nil {
		t.Fatal("supported legacy version returned an error")
	}
	if err := responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("3", "initialize", `{"protocolVersion":"unsupported"}`), RequestMeta{})); err.Code != UnsupportedVersion {
		t.Fatalf("unsupported initialize code = %d", err.Code)
	}
	if err := responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("4", "initialize", `{}`), RequestMeta{})); err.Code != InvalidParams {
		t.Fatalf("missing initialize version code = %d", err.Code)
	}
	if err := responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("5", "initialize", `null`), RequestMeta{})); err.Code != InvalidParams {
		t.Fatalf("invalid initialize params code = %d", err.Code)
	}

	ping := responseResult[map[string]any](t, handler.Dispatch(context.Background(), Principal{}, testRequest("6", "ping", ""), RequestMeta{}))
	if len(ping) != 0 {
		t.Fatalf("ping result = %#v", ping)
	}
	if handler.Dispatch(context.Background(), Principal{}, testRequest("", "notifications/initialized", ""), RequestMeta{}) != nil {
		t.Fatal("initialized notification should not return a response")
	}
	tools := responseResult[ListToolsResult](t, handler.Dispatch(context.Background(), Principal{Scopes: []Scope{ScopeKnowledgeRead}}, testRequest("7", "tools/list", ""), RequestMeta{}))
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "read" {
		t.Fatalf("tools/list result = %#v", tools)
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("8", "tools/list", `[]`), RequestMeta{})).Code != InvalidParams {
		t.Fatal("invalid tools/list params did not return InvalidParams")
	}
	resources := responseResult[ListResourcesResult](t, handler.Dispatch(context.Background(), Principal{Scopes: []Scope{ScopeKnowledgeRead}}, testRequest("9", "resources/list", `{}`), RequestMeta{}))
	if len(resources.Resources) != 1 || resources.Resources[0].URI != "urn:read" {
		t.Fatalf("resources/list result = %#v", resources)
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("10", "resources/list", `null`), RequestMeta{})).Code != InvalidParams {
		t.Fatal("invalid resources/list params did not return InvalidParams")
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("11", "unknown", `{}`), RequestMeta{})).Code != MethodNotFound {
		t.Fatal("unknown method did not return MethodNotFound")
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, Request{JSONRPC: "1.0", ID: json.RawMessage("12"), Method: "ping"}, RequestMeta{})).Code != InvalidRequest {
		t.Fatal("invalid JSON-RPC envelope did not return InvalidRequest")
	}

	var nilHandler *Handler
	if responseError(t, nilHandler.Dispatch(context.Background(), Principal{}, testRequest("13", "ping", ""), RequestMeta{})).Code != InternalError {
		t.Fatal("nil handler did not fail closed")
	}
	if responseError(t, (&Handler{}).Dispatch(context.Background(), Principal{}, testRequest("14", "ping", ""), RequestMeta{})).Code != InternalError {
		t.Fatal("handler without registry did not fail closed")
	}
	if objectParamsResponse := handler.Dispatch(context.Background(), Principal{}, testRequest("15", "tools/list", `{"bad":1}`), RequestMeta{}); objectParamsResponse == nil || objectParamsResponse.Error != nil {
		// Any object is valid for tools/list; this branch documents that it remains accepted.
		t.Fatal("object tools/list params should be accepted")
	}

	var nilClock Handler
	if nilClock.now().IsZero() {
		t.Fatal("nil Handler clock should fall back to the system clock")
	}
}

func TestToolDispatchNonceIdempotencyReplayAndErrors(t *testing.T) {
	registry := NewRegistry()
	var readCalls atomic.Int32
	var mutateCalls atomic.Int32
	var blockCalls atomic.Int32
	var writeErrorCalls atomic.Int32
	var invocationRequestID string
	readHandler := func(_ context.Context, invocation Invocation) (CallToolResult, error) {
		readCalls.Add(1)
		invocationRequestID = string(invocation.RequestID)
		if len(invocation.Arguments) == 0 {
			t.Fatal("tool invocation did not receive normalized arguments")
		}
		return CallToolResult{Content: []Content{{Type: "text", Text: "read"}}}, nil
	}
	mutateHandler := func(_ context.Context, invocation Invocation) (CallToolResult, error) {
		mutateCalls.Add(1)
		return CallToolResult{Content: []Content{{Type: "text", Text: string(invocation.Arguments)}}}, nil
	}
	started := make(chan struct{})
	release := make(chan struct{})
	blockHandler := func(_ context.Context, _ Invocation) (CallToolResult, error) {
		blockCalls.Add(1)
		close(started)
		<-release
		return CallToolResult{Content: []Content{{Type: "text", Text: "block"}}}, nil
	}
	for _, tool := range []Tool{
		{Name: "read", Handler: readHandler},
		{Name: "mutate", Mutating: true, RequiredScope: ScopeWorkWrite, Handler: mutateHandler},
		{Name: "block", Mutating: true, RequiredScope: ScopeGitHubWrite, Handler: blockHandler},
		{Name: "rpc-error", Handler: func(context.Context, Invocation) (CallToolResult, error) {
			return CallToolResult{}, newRPCError(ForbiddenError, "denied", ErrMissingScope, map[string]string{"scope": "test"})
		}},
		{Name: "write-error", Mutating: true, RequiredScope: ScopeWorkWrite, Handler: func(context.Context, Invocation) (CallToolResult, error) {
			writeErrorCalls.Add(1)
			return CallToolResult{}, newRPCError(InternalError, "write failed", errors.New("write cause"), map[string]string{"reason": "test"})
		}},
		{Name: "error", Handler: func(context.Context, Invocation) (CallToolResult, error) {
			return CallToolResult{}, errors.New("tool failure")
		}},
		{Name: "bad-result", Handler: func(context.Context, Invocation) (CallToolResult, error) {
			return CallToolResult{StructuredContent: func() {}}, nil
		}},
	} {
		if err := registry.RegisterTool(tool); err != nil {
			t.Fatalf("RegisterTool(%q) error = %v", tool.Name, err)
		}
	}
	handler := NewHandler(registry, nil)
	principal := Principal{TokenID: "subject", Scopes: []Scope{ScopeWorkWrite, ScopeGitHubWrite}}

	readResponse := mustResponse(t, handler.Dispatch(context.Background(), principal, testRequest("1", "tools/call", `{"name":"read"}`), RequestMeta{}))
	responseResult[CallToolResult](t, readResponse)
	if readCalls.Load() != 1 || invocationRequestID != "1" {
		t.Fatal("read-only tool was not dispatched with its request ID")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("2", "tools/call", `{"name":"read","arguments":[]}`), RequestMeta{})).Code != InvalidParams {
		t.Fatal("array tool arguments did not return InvalidParams")
	}
	for _, request := range []Request{
		testRequest("3", "tools/call", `{}`),
		testRequest("4", "tools/call", `null`),
		testRequest("5", "tools/call", `{"name":"missing"}`),
	} {
		if responseError(t, handler.Dispatch(context.Background(), principal, request, RequestMeta{})).Code == 0 {
			t.Fatal("invalid tool call unexpectedly returned success")
		}
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("6", "tools/call", `{"name":"mutate"}`), RequestMeta{})).Code != ForbiddenError {
		t.Fatal("missing tool scope did not return ForbiddenError")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("7", "tools/call", `{"name":"mutate"}`), RequestMeta{})).Code != InvalidParams {
		t.Fatal("missing idempotency metadata did not return InvalidParams")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("8", "tools/call", `{"name":"mutate"}`), RequestMeta{IdempotencyKey: "key"})).Code != InvalidParams {
		t.Fatal("missing nonce did not return InvalidParams")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("9", "tools/call", `{"name":"mutate"}`), RequestMeta{IdempotencyKey: " ", Nonce: "nonce"})).Code != InvalidParams {
		t.Fatal("invalid idempotency key did not return InvalidParams")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("10", "tools/call", `{"name":"mutate"}`), RequestMeta{IdempotencyKey: "key", Nonce: "\t"})).Code != InvalidParams {
		t.Fatal("invalid nonce did not return InvalidParams")
	}

	mutateParams := `{"name":"mutate","arguments":{"value":"one"}}`
	first := mustResponse(t, handler.Dispatch(context.Background(), principal, testRequest("11", "tools/call", mutateParams), RequestMeta{IdempotencyKey: "mutate-key", Nonce: "mutate-nonce"}))
	if first.Error != nil || mutateCalls.Load() != 1 {
		t.Fatal("first mutating call did not execute successfully")
	}
	retry := mustResponse(t, handler.Dispatch(context.Background(), principal, testRequest("12", "tools/call", mutateParams), RequestMeta{IdempotencyKey: "mutate-key", Nonce: "different-nonce"}))
	if retry.Error != nil || string(retry.ID) != "12" || !bytes.Equal(retry.Result, first.Result) || mutateCalls.Load() != 1 {
		t.Fatal("idempotency replay did not correlate the current request ID")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("13", "tools/call", `{"name":"mutate","arguments":{"value":"two"}}`), RequestMeta{IdempotencyKey: "mutate-key", Nonce: "new-nonce"})).Code != IdempotencyError {
		t.Fatal("idempotency conflict did not return IdempotencyError")
	}

	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("14", "tools/call", `{"name":"rpc-error"}`), RequestMeta{})).Code != ForbiddenError {
		t.Fatal("RPCError tool result was not preserved")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("15", "tools/call", `{"name":"error"}`), RequestMeta{})).Code != InternalError {
		t.Fatal("ordinary tool error was not mapped to InternalError")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("16", "tools/call", `{"name":"bad-result"}`), RequestMeta{})).Code != InternalError {
		t.Fatal("unmarshalable tool result was not mapped to InternalError")
	}
	writeErrorParams := `{"name":"write-error","arguments":{"value":"one"}}`
	firstWriteError := responseError(t, handler.Dispatch(context.Background(), principal, testRequest(`"16a"`, "tools/call", writeErrorParams), RequestMeta{IdempotencyKey: "write-error-key", Nonce: "write-error-nonce"}))
	secondWriteError := responseError(t, handler.Dispatch(context.Background(), principal, testRequest(`"16b"`, "tools/call", writeErrorParams), RequestMeta{IdempotencyKey: "write-error-key", Nonce: "write-error-retry-nonce"}))
	firstWriteErrorData, err := json.Marshal(firstWriteError.Data)
	if err != nil {
		t.Fatalf("encode first cached error data: %v", err)
	}
	secondWriteErrorData, err := json.Marshal(secondWriteError.Data)
	if err != nil {
		t.Fatalf("encode replayed error data: %v", err)
	}
	if secondWriteError.Code != firstWriteError.Code || secondWriteError.Message != firstWriteError.Message || !bytes.Equal(secondWriteErrorData, firstWriteErrorData) || writeErrorCalls.Load() != 1 {
		t.Fatal("idempotency replay did not retain the cached error payload")
	}
	if replayErrorResponse := handler.Dispatch(context.Background(), principal, testRequest(`"16c"`, "tools/call", writeErrorParams), RequestMeta{IdempotencyKey: "write-error-key", Nonce: "another-retry-nonce"}); replayErrorResponse == nil || string(replayErrorResponse.ID) != `"16c"` {
		t.Fatal("cached error replay did not use the current request ID")
	}

	if setupResponse := handler.Dispatch(context.Background(), principal, testRequest("17", "tools/call", `{"name":"mutate"}`), RequestMeta{IdempotencyKey: "nonce-key", Nonce: "used-nonce"}); setupResponse == nil || setupResponse.Error != nil {
		t.Fatal("nonce setup call failed")
	}
	if responseError(t, handler.Dispatch(context.Background(), principal, testRequest("18", "tools/call", `{"name":"mutate"}`), RequestMeta{IdempotencyKey: "nonce-key-2", Nonce: "used-nonce"})).Code != ReplayError {
		t.Fatal("duplicate nonce did not return ReplayError")
	}
	if recoveredResponse := handler.Dispatch(context.Background(), principal, testRequest("19", "tools/call", `{"name":"mutate"}`), RequestMeta{IdempotencyKey: "nonce-key-2", Nonce: "fresh-nonce"}); recoveredResponse == nil || recoveredResponse.Error != nil {
		t.Fatal("aborted idempotency reservation was not reusable")
	}

	firstBlockDone := make(chan *Response, 1)
	go func() {
		firstBlockDone <- handler.Dispatch(context.Background(), principal, testRequest("20", "tools/call", `{"name":"block"}`), RequestMeta{IdempotencyKey: "block-key", Nonce: "block-nonce"})
	}()
	<-started
	pending := responseError(t, handler.Dispatch(context.Background(), principal, testRequest("21", "tools/call", `{"name":"block"}`), RequestMeta{IdempotencyKey: "block-key", Nonce: "other-block-nonce"}))
	if pending.Code != IdempotencyError || !errors.Is(pending, ErrIdempotencyInProgress) {
		t.Fatalf("pending idempotency response = %#v", pending)
	}
	close(release)
	if response := <-firstBlockDone; response == nil || response.Error != nil {
		t.Fatal("blocked first call did not complete")
	}
	if blockCalls.Load() != 1 {
		t.Fatal("pending replay executed the blocked tool twice")
	}

	if notification := handler.Dispatch(context.Background(), principal, testRequest("", "tools/call", `{"name":"read"}`), RequestMeta{}); notification != nil {
		t.Fatal("tool notification should not return a response")
	}
}

func TestResourceDispatchNormalizationAndHTTPTransport(t *testing.T) {
	registry := NewRegistry()
	text := "hello"
	if err := registry.RegisterResource(Resource{URI: "urn:text", RequiredScope: ScopeKnowledgeRead, Reader: func(context.Context, ResourceRequest) ([]ResourceContent, error) {
		return []ResourceContent{{Text: &text}}, nil
	}}); err != nil {
		t.Fatalf("text resource registration error = %v", err)
	}
	if err := registry.RegisterResource(Resource{URI: "urn:error", Reader: func(context.Context, ResourceRequest) ([]ResourceContent, error) {
		return nil, errors.New("resource failure")
	}}); err != nil {
		t.Fatalf("error resource registration error = %v", err)
	}
	if err := registry.RegisterResource(Resource{URI: "urn:bad", Reader: func(context.Context, ResourceRequest) ([]ResourceContent, error) {
		return []ResourceContent{{URI: "urn:other", Text: &text}}, nil
	}}); err != nil {
		t.Fatalf("bad resource registration error = %v", err)
	}
	handler := NewHandler(registry, nil)

	valid := responseResult[ReadResourceResult](t, handler.Dispatch(context.Background(), Principal{Scopes: []Scope{ScopeKnowledgeRead}}, testRequest("1", "resources/read", `{"uri":"urn:text"}`), RequestMeta{}))
	if len(valid.Contents) != 1 || valid.Contents[0].URI != "urn:text" || valid.Contents[0].Text == nil {
		t.Fatalf("valid resource result = %#v", valid)
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("2", "resources/read", `{"uri":"urn:text"}`), RequestMeta{})).Code != ForbiddenError {
		t.Fatal("resource scope denial did not return ForbiddenError")
	}
	for _, request := range []Request{
		testRequest("3", "resources/read", `{}`),
		testRequest("4", "resources/read", `{"uri":"urn:missing"}`),
		testRequest("5", "resources/read", `null`),
	} {
		if responseError(t, handler.Dispatch(context.Background(), Principal{Scopes: []Scope{ScopeKnowledgeRead}}, request, RequestMeta{})).Code == 0 {
			t.Fatal("invalid resource read unexpectedly succeeded")
		}
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("6", "resources/read", `{"uri":"urn:error"}`), RequestMeta{})).Code != InternalError {
		t.Fatal("resource reader error did not return InternalError")
	}
	if responseError(t, handler.Dispatch(context.Background(), Principal{}, testRequest("7", "resources/read", `{"uri":"urn:bad"}`), RequestMeta{})).Code != InternalError {
		t.Fatal("invalid resource content did not return InternalError")
	}

	blob := []byte{1, 2, 3}
	contents, err := normalizeResourceContents("urn:blob", []ResourceContent{{Blob: blob}})
	if err != nil || !bytes.Equal(contents[0].Blob, blob) || &contents[0].Blob[0] == &blob[0] {
		t.Fatal("blob resource content was not normalized and copied")
	}
	if _, err := normalizeResourceContents("urn:test", []ResourceContent{{Text: &text, Blob: blob}}); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("text/blob resource error = %v", err)
	}
	if _, err := normalizeResourceContents("urn:test", []ResourceContent{{}}); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("empty resource content error = %v", err)
	}
	if _, err := normalizeResourceContents("urn:test", []ResourceContent{{URI: "urn:other", Text: &text}}); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("mismatched resource URI error = %v", err)
	}
	if _, err := normalizeResourceContents("urn:test", nil); err != nil {
		t.Fatalf("empty resource content list error = %v", err)
	}

	store := NewTokenStore(WithTokenRandom(&incrementingReader{}))
	issued, err := store.Issue([]Scope{ScopeKnowledgeRead})
	if err != nil {
		t.Fatalf("HTTP token Issue() error = %v", err)
	}
	token := mustReveal(t, &issued)
	httpHandler := NewHandler(registry, store, WithMaxBodyBytes(128))
	getRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if getRecorder.Code != http.StatusMethodNotAllowed || getRecorder.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET HTTP status/allow = %d/%q", getRecorder.Code, getRecorder.Header().Get("Allow"))
	}
	unauthorizedRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(unauthorizedRecorder, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`)))
	if unauthorizedRecorder.Code != http.StatusUnauthorized || unauthorizedRecorder.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("unauthorized HTTP status/header = %d/%q", unauthorizedRecorder.Code, unauthorizedRecorder.Header().Get("WWW-Authenticate"))
	}
	badAuthRecorder := httptest.NewRecorder()
	badAuthRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	badAuthRequest.Header.Set(AuthorizationHeader, "Basic value")
	httpHandler.ServeHTTP(badAuthRecorder, badAuthRequest)
	if badAuthRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("bad authorization HTTP status = %d", badAuthRecorder.Code)
	}
	missingAcceptRecorder := httptest.NewRecorder()
	missingAcceptRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	missingAcceptRequest.Header.Set(AuthorizationHeader, "Bearer "+token)
	httpHandler.ServeHTTP(missingAcceptRecorder, missingAcceptRequest)
	if missingAcceptRecorder.Code != http.StatusNotAcceptable {
		t.Fatalf("missing Accept HTTP status = %d", missingAcceptRecorder.Code)
	}
	partialAcceptRecorder := httptest.NewRecorder()
	partialAcceptRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	partialAcceptRequest.Header.Set(AuthorizationHeader, "Bearer "+token)
	partialAcceptRequest.Header.Set(AcceptHeader, "application/json")
	httpHandler.ServeHTTP(partialAcceptRecorder, partialAcceptRequest)
	if partialAcceptRecorder.Code != http.StatusNotAcceptable {
		t.Fatalf("partial Accept HTTP status = %d", partialAcceptRecorder.Code)
	}
	invalidJSONRecorder := httptest.NewRecorder()
	invalidJSONRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("not-json"))
	setMCPRequestHeaders(invalidJSONRequest, token)
	httpHandler.ServeHTTP(invalidJSONRecorder, invalidJSONRequest)
	if invalidJSONRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON HTTP status = %d", invalidJSONRecorder.Code)
	}
	largeRecorder := httptest.NewRecorder()
	largeRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(strings.Repeat("x", 256)))
	setMCPRequestHeaders(largeRequest, token)
	httpHandler.ServeHTTP(largeRecorder, largeRequest)
	if largeRecorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large HTTP status = %d", largeRecorder.Code)
	}
	validRecorder := httptest.NewRecorder()
	validRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`))
	setMCPRequestHeaders(validRequest, token)
	httpHandler.ServeHTTP(validRecorder, validRequest)
	if validRecorder.Code != http.StatusOK || validRecorder.Header().Get("Content-Type") != "application/json" || validRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("valid HTTP response = %d, headers=%v", validRecorder.Code, validRecorder.Header())
	}
	notificationRecorder := httptest.NewRecorder()
	notificationRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	setMCPRequestHeaders(notificationRequest, token)
	httpHandler.ServeHTTP(notificationRecorder, notificationRequest)
	if notificationRecorder.Code != http.StatusAccepted {
		t.Fatalf("notification HTTP status = %d", notificationRecorder.Code)
	}
	if notificationRecorder.Body.Len() != 0 {
		t.Fatalf("notification response body = %q, want empty", notificationRecorder.Body.String())
	}
	unsupportedVersionRecorder := httptest.NewRecorder()
	unsupportedVersionRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	setMCPRequestHeaders(unsupportedVersionRequest, token)
	unsupportedVersionRequest.Header.Set(ProtocolVersionHeader, "2099-01-01")
	httpHandler.ServeHTTP(unsupportedVersionRecorder, unsupportedVersionRequest)
	if unsupportedVersionRecorder.Code != http.StatusBadRequest {
		t.Fatalf("unsupported protocol version HTTP status = %d", unsupportedVersionRecorder.Code)
	}
	var unsupportedVersionResponse Response
	if err := json.Unmarshal(unsupportedVersionRecorder.Body.Bytes(), &unsupportedVersionResponse); err != nil {
		t.Fatalf("decode unsupported version response: %v", err)
	}
	if unsupportedVersionResponse.Error == nil || unsupportedVersionResponse.Error.Code != UnsupportedVersion {
		t.Fatalf("unsupported protocol version response = %#v", unsupportedVersionResponse)
	}
	supportedVersionRecorder := httptest.NewRecorder()
	supportedVersionRequest := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	setMCPRequestHeaders(supportedVersionRequest, token)
	supportedVersionRequest.Header.Set(ProtocolVersionHeader, "2025-03-26")
	httpHandler.ServeHTTP(supportedVersionRecorder, supportedVersionRequest)
	if supportedVersionRecorder.Code != http.StatusOK {
		t.Fatalf("supported protocol version HTTP status = %d", supportedVersionRecorder.Code)
	}
	serviceRecorder := httptest.NewRecorder()
	NewHandler(registry, nil).ServeHTTP(serviceRecorder, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`)))
	if serviceRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing token store HTTP status = %d", serviceRecorder.Code)
	}
	var nilHandler *Handler
	nilRecorder := httptest.NewRecorder()
	nilHandler.ServeHTTP(nilRecorder, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`)))
	if nilRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil handler HTTP status = %d", nilRecorder.Code)
	}
}

func TestAcceptNegotiationRequiresJSONAndSSE(t *testing.T) {
	valid := []string{
		"application/json, text/event-stream",
		"text/event-stream; q=0.5, application/json; q=1",
		"APPLICATION/JSON, TEXT/EVENT-STREAM",
	}
	for _, value := range valid {
		if !acceptsRequiredResponseTypes(value) {
			t.Errorf("acceptsRequiredResponseTypes(%q) = false, want true", value)
		}
	}
	invalid := []string{"", "application/json", "text/event-stream", "*/*", "application/*, text/event-stream"}
	for _, value := range invalid {
		if acceptsRequiredResponseTypes(value) {
			t.Errorf("acceptsRequiredResponseTypes(%q) = true, want false", value)
		}
	}
}

func TestErrorMappingAndPrivateHelpers(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{ErrInvalidRequest, InvalidRequest},
		{ErrInvalidParams, InvalidParams},
		{ErrMethodNotFound, MethodNotFound},
		{ErrToolNotFound, NotFoundError},
		{ErrResourceNotFound, NotFoundError},
		{ErrMissingScope, ForbiddenError},
		{ErrReplayDetected, ReplayError},
		{ErrIdempotencyConflict, IdempotencyError},
		{ErrIdempotencyInProgress, IdempotencyError},
		{ErrMissingIdempotencyKey, InvalidParams},
		{ErrMissingNonce, InvalidParams},
		{ErrInvalidIdempotencyKey, InvalidParams},
		{ErrInvalidNonce, InvalidParams},
		{ErrUnsupportedVersion, UnsupportedVersion},
		{ErrInvalidToken, InternalError},
	}
	for _, testCase := range tests {
		rpcError := errorToRPC(testCase.err)
		if rpcError.Code != testCase.code || rpcError.Message == "" || !errors.Is(rpcError, testCase.err) {
			t.Errorf("errorToRPC(%v) = %#v", testCase.err, rpcError)
		}
	}
	if errorToRPC(nil) != nil {
		t.Fatal("errorToRPC(nil) should return nil")
	}
	existing := newRPCError(123, "existing", errors.New("cause"), map[string]string{"safe": "data"})
	if got := errorToRPC(fmt.Errorf("wrapped: %w", existing)); got != existing {
		t.Fatal("errorToRPC() did not preserve an existing RPCError")
	}

	for _, raw := range [][]byte{
		[]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`),
		[]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"failure"}}`),
	} {
		if _, err := decodeResponse(raw); err != nil {
			t.Fatalf("valid decodeResponse() error = %v", err)
		}
	}
	for _, raw := range [][]byte{
		[]byte("not-json"),
		[]byte(`{"jsonrpc":"1.0","id":1,"result":{}}`),
		[]byte(`{"jsonrpc":"2.0","id":1}`),
	} {
		if _, err := decodeResponse(raw); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("invalid decodeResponse() error = %v", err)
		}
	}
	request := Request{}
	if !request.IsNotification() || finishResponse(request, nil) != nil {
		t.Fatal("notification response handling is incorrect")
	}
	if finishResponse(testRequest("1", "ping", ""), nil) != nil {
		t.Fatal("non-notification nil response should remain nil")
	}
	if err := validateName(""); !errors.Is(err, ErrInvalidTool) || validateName("ok") != nil {
		t.Fatal("validateName() basic cases are incorrect")
	}
	if err := validateRequiredScope(Scope("unknown")); !errors.Is(err, ErrUnknownScope) || validateRequiredScope("") != nil {
		t.Fatal("validateRequiredScope() cases are incorrect")
	}
	if !isControl('\n') || !isControl(0x7f) || isControl('a') {
		t.Fatal("isControl() cases are incorrect")
	}
	if !bytes.Equal(cloneRaw(json.RawMessage("1")), json.RawMessage("1")) {
		t.Fatal("cloneRaw() changed data")
	}
}
