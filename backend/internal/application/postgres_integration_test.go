package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/testkit"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

// TestPostgresApplicationManualImportIsIdempotent exercises the composition
// root against the same PostgreSQL schema used by the local Docker runtime.
// The normal unit suite skips this test unless the operator opts into the
// disposable DEVENGLISH_TEST_DATABASE_URL fixture.
func TestPostgresApplicationManualImportIsIdempotent(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "application-it-user-" + suffix
	workspaceID := WorkspaceIDForUser(userID)
	ctx := store.WithUser(context.Background(), userID)
	legacyStore := &store.PostgresStore{Pool: fixture.Pool}
	if err := legacyStore.EnsureUser(ctx, domain.User{ID: userID, DisplayName: "Application integration", CEFR: "B1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed application integration user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// ImportManualSource creates only this source graph and its workspace;
		// remove children before the workspace because the migration chain uses
		// RESTRICT for canonical knowledge roots.
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM knowledge_chunks WHERE workspace_id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM source_revisions WHERE workspace_id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM source_items WHERE workspace_id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM knowledge_sources WHERE workspace_id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	app, err := NewPostgres(fixture.Pool)
	if err != nil {
		t.Fatalf("create PostgreSQL application: %v", err)
	}
	input := ManualSourceInput{
		Name:    "Application runbook",
		Kind:    "runbook",
		URI:     "memory://application-runbook-" + suffix,
		Content: "The release rollback remains reversible.",
	}
	idempotencyKey := "manual-import-" + suffix
	first, err := app.ImportManualSource(ctx, input, idempotencyKey)
	if err != nil {
		t.Fatalf("first manual import: %v", err)
	}
	before, err := app.GetKnowledgeSourceDetail(ctx, first.Source.ID)
	if err != nil {
		t.Fatalf("read first source detail: %v", err)
	}
	second, err := app.ImportManualSource(ctx, input, idempotencyKey)
	if err != nil {
		t.Fatalf("replay manual import: %v", err)
	}
	after, err := app.GetKnowledgeSourceDetail(ctx, first.Source.ID)
	if err != nil {
		t.Fatalf("read replayed source detail: %v", err)
	}
	if first.Source.ID != second.Source.ID || first.Source.Title != second.Source.Title || first.Source.Kind != second.Source.Kind || first.Source.URI != second.Source.URI || first.RevisionID != second.RevisionID || first.ChunkID != second.ChunkID || !first.Source.UpdatedAt.Equal(second.Source.UpdatedAt) {
		t.Fatalf("replay changed the imported identity: first=%+v second=%+v", first, second)
	}
	if len(after.Revisions) != 1 || len(after.Chunks) != 1 {
		t.Fatalf("replay created duplicate immutable rows: revisions=%d chunks=%d", len(after.Revisions), len(after.Chunks))
	}
	if len(before.Items) != 1 || len(after.Items) != 1 || before.Items[0].Version != after.Items[0].Version {
		t.Fatalf("replay changed source item version: before=%+v after=%+v", before.Items, after.Items)
	}
	restarted, err := NewPostgres(fixture.Pool)
	if err != nil {
		t.Fatalf("recreate PostgreSQL application: %v", err)
	}
	afterRestart, err := restarted.ImportManualSource(ctx, input, idempotencyKey)
	if err != nil {
		t.Fatalf("replay after application restart: %v", err)
	}
	if afterRestart != first {
		t.Fatalf("durable replay changed after application restart: first=%+v replay=%+v", first, afterRestart)
	}
	differentInput := input
	differentInput.Content = "The release rollback was replaced."
	if _, err := restarted.ImportManualSource(ctx, differentInput, idempotencyKey); !errors.Is(err, knowledge.ErrConflict) {
		t.Fatalf("same key with a different payload error = %v, want knowledge conflict", err)
	}
	asked, err := app.Ask(ctx, "", "What does the release rollback require?")
	if err != nil {
		t.Fatalf("ask against PostgreSQL application: %v", err)
	}
	if asked.ConversationID == "" || asked.Response.Grounding != assistant.Grounded || len(asked.Response.Evidence) != 1 || asked.Response.Evidence[0].EvidenceID != "knowledge-"+first.ChunkID {
		t.Fatalf("PostgreSQL application response = %+v", asked)
	}
	bootstrapped, err := app.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("bootstrap persisted PostgreSQL conversation: %v", err)
	}
	if bootstrapped.Conversation == nil || len(bootstrapped.Conversation.Messages) != 2 || bootstrapped.Conversation.Messages[1].Response == nil {
		t.Fatalf("PostgreSQL conversation was not persisted: %+v", bootstrapped.Conversation)
	}
}

func TestPostgresApplicationManualImportConcurrentKeyCommitsOneSourceGraph(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "application-concurrent-import-user-" + suffix
	workspaceID := WorkspaceIDForUser(userID)
	ctx := store.WithUser(context.Background(), userID)
	legacyStore := &store.PostgresStore{Pool: fixture.Pool}
	if err := legacyStore.EnsureUser(ctx, domain.User{ID: userID, DisplayName: "Concurrent import", CEFR: "B1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed concurrent import user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	app, err := NewPostgres(fixture.Pool)
	if err != nil {
		t.Fatalf("create PostgreSQL application: %v", err)
	}
	input := ManualSourceInput{
		Name:    "Concurrent application runbook",
		Kind:    "runbook",
		URI:     "memory://application-concurrent-" + suffix,
		Content: "Concurrent requests must converge on one committed source graph.",
	}
	const callers = 8
	results := make(chan ImportedSource, callers)
	errorsCh := make(chan error, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, importErr := app.ImportManualSource(ctx, input, "concurrent-import-"+suffix)
			if importErr != nil {
				errorsCh <- importErr
				return
			}
			results <- result
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errorsCh)
	for importErr := range errorsCh {
		t.Fatalf("concurrent PostgreSQL manual import: %v", importErr)
	}
	if len(results) != callers {
		t.Fatalf("completed PostgreSQL imports = %d, want %d", len(results), callers)
	}
	var first ImportedSource
	index := 0
	for result := range results {
		if index == 0 {
			first = result
			index++
			continue
		}
		if result != first {
			t.Fatalf("concurrent PostgreSQL replay changed projection: first=%+v result=%+v", first, result)
		}
		index++
	}
	detail, err := app.GetKnowledgeSourceDetail(ctx, first.Source.ID)
	if err != nil {
		t.Fatalf("read concurrent imported source: %v", err)
	}
	if len(detail.Items) != 1 || len(detail.Revisions) != 1 || len(detail.Chunks) != 1 {
		t.Fatalf("concurrent PostgreSQL import created duplicate graph rows: items=%d revisions=%d chunks=%d", len(detail.Items), len(detail.Revisions), len(detail.Chunks))
	}
	var idempotencyRows int
	if err := fixture.Pool.QueryRow(ctx, `
		SELECT count(*) FROM knowledge_import_idempotency
		WHERE workspace_id=$1 AND user_id=$2 AND idempotency_key=$3
	`, workspaceID, userID, "concurrent-import-"+suffix).Scan(&idempotencyRows); err != nil {
		t.Fatalf("count concurrent idempotency rows: %v", err)
	}
	if idempotencyRows != 1 {
		t.Fatalf("concurrent idempotency rows = %d, want 1", idempotencyRows)
	}
}

func TestPostgresConversationRepositoryPersistsScopedHistory(t *testing.T) {
	fixture := testkit.RequirePostgres(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "conversation-it-user-" + suffix
	workspaceID := WorkspaceIDForUser(userID)
	ctx := store.WithUser(context.Background(), userID)
	legacyStore := &store.PostgresStore{Pool: fixture.Pool}
	if err := legacyStore.EnsureUser(ctx, domain.User{ID: userID, DisplayName: "Conversation integration", CEFR: "B1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed conversation integration user: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
		_, _ = fixture.Pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID)
	})

	workspaceRepository := &PostgresWorkspaceRepository{Pool: fixture.Pool}
	if err := workspaceRepository.EnsureWorkspace(ctx, workspaceID, userID); err != nil {
		t.Fatalf("ensure conversation workspace: %v", err)
	}
	repository, err := NewPostgresConversationRepository(fixture.Pool)
	if err != nil {
		t.Fatalf("create conversation repository: %v", err)
	}
	scope := work.Scope{WorkspaceID: workspaceID, UserID: userID}
	contextRef := assistant.ContextRef{Type: assistant.ContextTask, ID: "conversation-context-task"}
	conversation, err := repository.Create(ctx, scope, "First line\nAdditional context", contextRef)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if conversation.Title != "First line" || len(conversation.Messages) != 0 || conversation.Context == nil || *conversation.Context != contextRef {
		t.Fatalf("created conversation = %+v, want first-line title and no messages", conversation)
	}
	response := assistant.AssistantResponse{
		Scope:     assistant.Scope{WorkspaceID: workspaceID, UserID: userID},
		Answer:    "Use the verified runbook.",
		Grounding: assistant.Grounded,
		Evidence: []assistant.Citation{{
			EvidenceID: "knowledge-chunk-1",
			Quote:      "The verified runbook is current.",
			Locator:    "runbook.md#rollback",
		}},
		Unknowns:         []string{"The live provider latency is unknown."},
		StaleSources:     []string{},
		SuggestedActions: []assistant.SuggestedAction{},
		ActionReceipts:   []assistant.ActionReceipt{},
	}
	if err := repository.Append(ctx, scope, conversation.ID, "What is the safe next step?", response); err != nil {
		t.Fatalf("append conversation: %v", err)
	}

	loaded, err := repository.Get(ctx, scope, conversation.ID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if len(loaded.Messages) != 2 || loaded.Messages[0].Role != "user" || loaded.Messages[1].Role != "assistant" {
		t.Fatalf("loaded messages = %+v, want user and assistant turns", loaded.Messages)
	}
	if loaded.Messages[1].Response == nil || loaded.Messages[1].Response.Grounding != assistant.Grounded || len(loaded.Messages[1].Response.Evidence) != 1 {
		t.Fatalf("loaded assistant response = %+v, want persisted grounding and citation", loaded.Messages[1].Response)
	}
	if loaded.Context == nil || *loaded.Context != contextRef {
		t.Fatalf("loaded conversation context = %+v, want %+v", loaded.Context, contextRef)
	}
	latest, err := repository.Get(ctx, scope, "latest")
	if err != nil || latest.ID != conversation.ID {
		t.Fatalf("latest conversation = %+v, err=%v", latest, err)
	}
	summaries, err := repository.List(ctx, scope, 20)
	if err != nil {
		t.Fatalf("list conversation summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != conversation.ID || summaries[0].MessageCount != 2 || summaries[0].Context == nil || *summaries[0].Context != contextRef {
		t.Fatalf("conversation summaries = %+v, want scoped metadata with two messages", summaries)
	}
	if _, err := repository.Get(ctx, work.Scope{WorkspaceID: workspaceID, UserID: "conversation-other-user-" + suffix}, conversation.ID); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("cross-user conversation error = %v, want not found", err)
	}
	otherSummaries, err := repository.List(ctx, work.Scope{WorkspaceID: workspaceID, UserID: "conversation-other-user-" + suffix}, 20)
	if err != nil || len(otherSummaries) != 0 {
		t.Fatalf("cross-user conversation summaries = %+v, err=%v, want empty", otherSummaries, err)
	}
	if _, err := repository.Get(ctx, scope, "missing-conversation"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("missing conversation error = %v, want not found", err)
	}
}
