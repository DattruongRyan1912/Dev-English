package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

type testWorkspaceEnsurer struct {
	err   error
	calls int
}

func (e *testWorkspaceEnsurer) EnsureWorkspace(context.Context, string, string) error {
	e.calls++
	return e.err
}

type testEmbeddingProvider struct {
	query         []float32
	document      []float32
	queryErr      error
	documentErr   error
	queryCalls    int
	documentCalls int
}

func (p *testEmbeddingProvider) Embed(context.Context, string) ([]float32, error) {
	p.queryCalls++
	return append([]float32(nil), p.query...), p.queryErr
}

func (p *testEmbeddingProvider) EmbedDocument(context.Context, string) ([]float32, error) {
	p.documentCalls++
	return append([]float32(nil), p.document...), p.documentErr
}

type testHybridRepository struct {
	*knowledge.MemoryRepository
	hybridErr     error
	hybridResults []knowledge.SearchResult
}

func (r *testHybridRepository) SearchHybrid(ctx context.Context, scope knowledge.WorkspaceScope, query string, _ []float32, limit int) ([]knowledge.SearchResult, error) {
	if r.hybridErr != nil {
		return nil, r.hybridErr
	}
	if r.hybridResults != nil {
		results := append([]knowledge.SearchResult(nil), r.hybridResults...)
		if len(results) > limit {
			results = results[:limit]
		}
		return results, nil
	}
	return r.Search(ctx, scope, query, limit)
}

type testAssistantGenerator struct {
	draft assistant.Draft
	err   error
}

func (g testAssistantGenerator) Generate(context.Context, assistant.GenerationRequest) (assistant.Draft, error) {
	return g.draft, g.err
}

func vector384(value float32) []float32 {
	result := make([]float32, knowledge.EmbeddingDimensions)
	result[0] = value
	return result
}

func appContext() context.Context {
	return store.WithUser(context.Background(), "user-1")
}

func TestAppBootstrapWorkKnowledgeSearchAndConversation(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()

	initial, err := app.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if initial.Workspace.ID != WorkspaceIDForUser("user-1") || initial.Conversation != nil {
		t.Fatalf("unexpected initial bootstrap: %+v", initial)
	}

	project, err := app.CreateProject(ctx, work.CreateProjectInput{
		ID:          "project-api",
		Name:        "API platform",
		Description: "Keep the rollback procedure reversible.",
	}, "project-create-1")
	if err != nil {
		t.Fatal(err)
	}
	task, err := app.CreateTask(ctx, work.CreateTaskInput{
		ID:          "task-rollback",
		ProjectID:   project.ID,
		Title:       "Document rollback",
		Description: "Verify the rollback procedure before release.",
		Priority:    work.PriorityHigh,
	}, "task-create-1")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := app.CreateDecision(ctx, work.CreateDecisionInput{
		ID:        "decision-storage",
		ProjectID: project.ID,
		Title:     "Use reversible migrations",
		Context:   "The API must recover from a failed deployment.",
		Outcome:   "Keep migration rollback available.",
		Rationale: "It reduces recovery risk.",
	}, "decision-create-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.ProjectID != project.ID || decision.ProjectID != project.ID {
		t.Fatalf("work links were not retained: task=%+v decision=%+v", task, decision)
	}

	imported, err := app.ImportManualSource(ctx, ManualSourceInput{
		Name:    "Release runbook",
		Kind:    "runbook",
		URI:     "memory://release-runbook",
		Content: "The rollback procedure is reversible and must be verified before release.",
	})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := app.SearchKnowledge(ctx, "rollback", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 3 {
		t.Fatalf("expected knowledge and work hits, got %+v", hits)
	}
	var knowledgeHit, projectHit, taskHit bool
	for _, hit := range hits {
		switch {
		case hit.ID == "knowledge-"+imported.ChunkID:
			knowledgeHit = hit.RetrievalMode == "lexical" && hit.Origin == "canonical"
		case hit.ID == "work-project-"+project.ID:
			projectHit = true
		case hit.ID == "work-task-"+task.ID:
			taskHit = true
		}
	}
	if !knowledgeHit || !projectHit || !taskHit {
		t.Fatalf("search did not preserve canonical hit types: %+v", hits)
	}

	detail, err := app.GetKnowledgeSourceDetail(ctx, imported.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Items) != 1 || len(detail.Revisions) != 1 || len(detail.Chunks) != 1 {
		t.Fatalf("unexpected source detail: %+v", detail)
	}

	asked, err := app.Ask(ctx, "", "What is the rollback procedure?")
	if err != nil {
		t.Fatal(err)
	}
	if asked.ConversationID == "" || asked.Response.Grounding != assistant.Grounded || len(asked.Response.Evidence) == 0 {
		t.Fatalf("assistant response was not grounded: %+v", asked)
	}

	continued, err := app.Ask(ctx, asked.ConversationID, "Summarize it.")
	if err != nil {
		t.Fatal(err)
	}
	if continued.ConversationID != asked.ConversationID {
		t.Fatalf("conversation was not continued: first=%q second=%q", asked.ConversationID, continued.ConversationID)
	}

	bootstrapped, err := app.BootstrapWithOptions(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if bootstrapped.Conversation == nil || len(bootstrapped.Conversation.Messages) != 4 {
		t.Fatalf("conversation was not persisted in bootstrap: %+v", bootstrapped.Conversation)
	}
}

func TestAppWrappersPreserveScopeAndConversationProjections(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()

	project, err := app.CreateProject(ctx, work.CreateProjectInput{
		ID: "wrapper-project", Name: "Wrapper project", Description: "A wrapper-scoped project.",
	}, "wrapper-project-create")
	if err != nil {
		t.Fatal(err)
	}
	task, err := app.CreateTask(ctx, work.CreateTaskInput{
		ID: "wrapper-task", ProjectID: project.ID, Title: "Wrapper task", Description: "A wrapper-scoped task.",
	}, "wrapper-task-create")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := app.CreateDecision(ctx, work.CreateDecisionInput{
		ID: "wrapper-decision", ProjectID: project.ID, Title: "Wrapper decision", Outcome: "Keep the wrapper boundary.",
	}, "wrapper-decision-create")
	if err != nil {
		t.Fatal(err)
	}
	if project.WorkspaceID != WorkspaceIDForUser("user-1") || task.OwnerUserID != "user-1" || decision.OwnerUserID != "user-1" {
		t.Fatalf("application wrappers escaped authenticated scope: project=%+v task=%+v decision=%+v", project, task, decision)
	}

	asked, err := app.Ask(ctx, "", "Explain the wrapper project")
	if err != nil {
		t.Fatal(err)
	}
	summaries, err := app.ListConversations(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].ID != asked.ConversationID || summaries[0].MessageCount != 2 || summaries[0].Title == "" {
		t.Fatalf("conversation list projection = %+v", summaries)
	}
	loaded, err := app.GetConversation(ctx, "  "+asked.ConversationID+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != asked.ConversationID || len(loaded.Messages) != 2 || loaded.Messages[1].Response == nil {
		t.Fatalf("conversation detail projection = %+v", loaded)
	}
	if _, err := app.GetConversation(ctx, "missing-conversation"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("missing conversation error = %v", err)
	}
	otherSummaries, err := app.ListConversations(store.WithUser(context.Background(), "user-2"), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherSummaries) != 0 {
		t.Fatalf("cross-user conversation list = %+v", otherSummaries)
	}

	scope, _, _, err := app.Scope(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Work.TrashProject(ctx, scope, project.ID, project.Version, "wrapper-project-trash"); err != nil {
		t.Fatal(err)
	}
	withoutTrashed, err := app.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	withTrashed, err := app.BootstrapWithOptions(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutTrashed.Projects) != 0 || len(withTrashed.Projects) != 1 || withTrashed.Projects[0].DeletedAt == nil {
		t.Fatalf("bootstrap trash filtering = without=%+v with=%+v", withoutTrashed.Projects, withTrashed.Projects)
	}
}

func TestAppWrappersPropagateWorkspaceInitializationFailure(t *testing.T) {
	workspaceErr := errors.New("workspace initialization failed")
	app, err := New(work.NewMemoryRepository(), knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), &testWorkspaceEnsurer{err: workspaceErr})
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	if _, err := app.CreateProject(ctx, work.CreateProjectInput{Name: "project"}, "project"); !errors.Is(err, workspaceErr) {
		t.Fatalf("CreateProject error = %v", err)
	}
	if _, err := app.CreateTask(ctx, work.CreateTaskInput{Title: "task"}, "task"); !errors.Is(err, workspaceErr) {
		t.Fatalf("CreateTask error = %v", err)
	}
	if _, err := app.CreateDecision(ctx, work.CreateDecisionInput{Title: "decision"}, "decision"); !errors.Is(err, workspaceErr) {
		t.Fatalf("CreateDecision error = %v", err)
	}
	if _, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "source", Content: "content"}); !errors.Is(err, workspaceErr) {
		t.Fatalf("ImportManualSource error = %v", err)
	}
	if _, err := app.SearchKnowledge(ctx, "query", 10); !errors.Is(err, workspaceErr) {
		t.Fatalf("SearchKnowledge error = %v", err)
	}
	if _, err := app.GetKnowledgeSourceDetail(ctx, "source"); !errors.Is(err, workspaceErr) {
		t.Fatalf("GetKnowledgeSourceDetail error = %v", err)
	}
	if _, err := app.ListConversations(ctx, 10); !errors.Is(err, workspaceErr) {
		t.Fatalf("ListConversations error = %v", err)
	}
	if _, err := app.GetConversation(ctx, "conversation"); !errors.Is(err, workspaceErr) {
		t.Fatalf("GetConversation error = %v", err)
	}
	if _, err := app.Ask(ctx, "", "hello"); !errors.Is(err, workspaceErr) {
		t.Fatalf("Ask error = %v", err)
	}
}

func TestAppAskWithContextPinsCanonicalEntityAndRejectsContextSwitch(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	project, err := app.CreateProject(ctx, work.CreateProjectInput{
		ID:          "context-project",
		Name:        "Context project",
		Description: "Keep the task grounded in the current work item.",
	}, "context-project-create")
	if err != nil {
		t.Fatal(err)
	}
	task, err := app.CreateTask(ctx, work.CreateTaskInput{
		ID:          "context-task",
		ProjectID:   project.ID,
		Title:       "Verify context grounding",
		Description: "Use the canonical task record when answering.",
		Priority:    work.PriorityHigh,
	}, "context-task-create")
	if err != nil {
		t.Fatal(err)
	}

	asked, err := app.AskWithContext(ctx, "", "What should I verify?", assistant.ContextRef{Type: " TASK ", ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if asked.ConversationID == "" || asked.Response.Grounding != assistant.Grounded || len(asked.Response.Evidence) == 0 || asked.Response.Evidence[0].EvidenceID != "work-task-"+task.ID {
		t.Fatalf("context ask was not grounded in the requested task: %+v", asked)
	}

	bootstrapped, err := app.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bootstrapped.Conversation == nil || bootstrapped.Conversation.Context == nil || *bootstrapped.Conversation.Context != (assistant.ContextRef{Type: assistant.ContextTask, ID: task.ID}) {
		t.Fatalf("conversation context was not persisted: %+v", bootstrapped.Conversation)
	}

	continued, err := app.Ask(ctx, asked.ConversationID, "Continue with this task.")
	if err != nil || continued.ConversationID != asked.ConversationID || continued.Response.Evidence[0].EvidenceID != "work-task-"+task.ID {
		t.Fatalf("stored context was not reused on continuation: result=%+v err=%v", continued, err)
	}
	if _, err := app.AskWithContext(ctx, asked.ConversationID, "Switch focus.", assistant.ContextRef{Type: assistant.ContextProject, ID: project.ID}); !errors.Is(err, ErrConversationContextConflict) {
		t.Fatalf("context switch error = %v, want %v", err, ErrConversationContextConflict)
	}
	finalBootstrap, err := app.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if finalBootstrap.Conversation == nil || len(finalBootstrap.Conversation.Messages) != 4 {
		t.Fatalf("rejected context switch should not append a message: %+v", finalBootstrap.Conversation)
	}
}

func TestAppAskWithContextRejectsMissingEntityBeforeCreatingConversation(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	if _, err := app.AskWithContext(ctx, "", "Check this task.", assistant.ContextRef{
		Type: assistant.ContextTask,
		ID:   "missing-task",
	}); !errors.Is(err, work.ErrNotFound) {
		t.Fatalf("missing context error = %v, want work.ErrNotFound", err)
	}
	bootstrap, err := app.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bootstrap.Conversation != nil {
		t.Fatalf("failed context validation should not create a conversation: %+v", bootstrap.Conversation)
	}
}

func TestAppSearchKeepsPinnedContextWithinEvidenceLimit(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("context-project-%02d", index)
		if _, err := app.CreateProject(ctx, work.CreateProjectInput{
			ID:          id,
			Name:        "Context project " + id,
			Description: "This project is part of the context search corpus.",
		}, "create-"+id); err != nil {
			t.Fatal(err)
		}
	}

	evidence, err := app.Search(ctx, assistant.RetrievalRequest{
		Scope: assistant.Scope{
			WorkspaceID: WorkspaceIDForUser("user-1"),
			UserID:      "user-1",
		},
		Query: "context corpus",
		Context: assistant.ContextRef{
			Type: assistant.ContextProject,
			ID:   "context-project-24",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 20 {
		t.Fatalf("Search returned %d evidence items, want the bounded maximum of 20", len(evidence))
	}
	if evidence[0].ID != "work-project-context-project-24" {
		t.Fatalf("pinned context was not first: %+v", evidence[0])
	}
}

func TestAppSearchResolvesDecisionAndSourceContext(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	project, err := app.CreateProject(ctx, work.CreateProjectInput{
		ID:   "context-decision-project",
		Name: "Decision context project",
	}, "context-decision-project-create")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := app.CreateDecision(ctx, work.CreateDecisionInput{
		ID:        "context-decision",
		ProjectID: project.ID,
		Title:     "Keep the source canonical",
		Context:   "The assistant must not invent work facts.",
		Outcome:   "Cite the source revision.",
		Rationale: "It preserves traceability.",
	}, "context-decision-create")
	if err != nil {
		t.Fatal(err)
	}
	source, err := app.ImportManualSource(ctx, ManualSourceInput{
		Name:    "Context runbook",
		Kind:    "runbook",
		URI:     "memory://context-runbook",
		Content: "The source revision is the canonical reference.",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name    string
		context assistant.ContextRef
		wantID  string
		wantIn  string
	}{
		{
			name:    "decision",
			context: assistant.ContextRef{Type: assistant.ContextDecision, ID: decision.ID},
			wantID:  "work-decision-" + decision.ID,
			wantIn:  "Cite the source revision.",
		},
		{
			name:    "source",
			context: assistant.ContextRef{Type: assistant.ContextSource, ID: source.Source.ID},
			wantID:  "knowledge-source-" + source.Source.ID,
			wantIn:  "canonical reference",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			evidence, err := app.Search(ctx, assistant.RetrievalRequest{
				Scope: assistant.Scope{
					WorkspaceID: WorkspaceIDForUser("user-1"),
					UserID:      "user-1",
				},
				Query:   "no matching search term",
				Context: testCase.context,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(evidence) == 0 || evidence[0].ID != testCase.wantID || !strings.Contains(evidence[0].Snippet, testCase.wantIn) {
				t.Fatalf("context evidence = %+v, want id %q containing %q", evidence, testCase.wantID, testCase.wantIn)
			}
		})
	}
}

func TestAppSearchBoundsPinnedSourceEvidence(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	content := strings.Repeat("verified source content ", 500)
	source, err := app.ImportManualSource(ctx, ManualSourceInput{
		Name:    "Long context source",
		Kind:    "runbook",
		URI:     "memory://long-context-source",
		Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}

	evidence, err := app.Search(ctx, assistant.RetrievalRequest{
		Scope: assistant.Scope{
			WorkspaceID: WorkspaceIDForUser("user-1"),
			UserID:      "user-1",
		},
		Query: "term-with-no-matching-record",
		Context: assistant.ContextRef{
			Type: assistant.ContextSource,
			ID:   source.Source.ID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].ID != "knowledge-source-"+source.Source.ID {
		t.Fatalf("source context evidence = %+v, want one canonical source hit", evidence)
	}
	if got := len([]rune(evidence[0].Snippet)); got != 8_000 {
		t.Fatalf("source context snippet length = %d, want bounded length 8000", got)
	}
}

func TestAppScopeAndConfigurationBoundaries(t *testing.T) {
	if _, err := New(nil, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil); err == nil {
		t.Fatal("New should reject incomplete dependencies")
	}
	if _, err := New(work.NewMemoryRepository(), nil, NewMemoryConversationRepository(), nil); err == nil {
		t.Fatal("New should reject a nil knowledge repository")
	}
	if _, err := New(work.NewMemoryRepository(), knowledge.NewMemoryRepository(), nil, nil); err == nil {
		t.Fatal("New should reject a nil conversation repository")
	}

	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := app.Scope(nil); err == nil {
		t.Fatal("Scope should reject a nil context")
	}
	if err := app.SetAssistantGenerator(nil); err == nil {
		t.Fatal("SetAssistantGenerator should reject nil")
	}
	if err := (*App)(nil).SetAssistantGenerator(testAssistantGenerator{}); err == nil {
		t.Fatal("a nil App should reject SetAssistantGenerator")
	}
	if WorkspaceIDForUser("user-1") == WorkspaceIDForUser("user-2") {
		t.Fatal("workspace IDs must be user-specific")
	}

	ensurer := &testWorkspaceEnsurer{err: errors.New("workspace unavailable")}
	app, err = New(work.NewMemoryRepository(), knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), ensurer)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := app.Scope(appContext()); !errors.Is(err, ensurer.err) || ensurer.calls != 1 {
		t.Fatalf("workspace ensure error was not propagated: err=%v calls=%d", err, ensurer.calls)
	}
	if err := app.EnsureDefaultScope(appContext()); !errors.Is(err, ensurer.err) {
		t.Fatalf("EnsureDefaultScope error = %v, want %v", err, ensurer.err)
	}
}

func TestAppManualImportIsIdempotentAndEmbedsOnlyValidVectors(t *testing.T) {
	repository := knowledge.NewMemoryRepository()
	app, err := New(work.NewMemoryRepository(), repository, NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	embedder := &testEmbeddingProvider{document: vector384(0.5)}
	app.SetKnowledgeEmbedder(embedder)
	ctx := appContext()
	input := ManualSourceInput{Name: "Architecture", URI: "memory://architecture", Content: "The platform uses a reversible migration."}
	first, err := app.ImportManualSource(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	firstDetail, err := app.GetKnowledgeSourceDetail(ctx, first.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.ImportManualSource(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || embedder.documentCalls != 2 {
		t.Fatalf("manual import was not deterministic: first=%+v second=%+v embed calls=%d", first, second, embedder.documentCalls)
	}
	detail, err := app.GetKnowledgeSourceDetail(ctx, first.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Chunks) != 1 || len(detail.Chunks[0].Embedding) != knowledge.EmbeddingDimensions {
		t.Fatalf("valid document embedding was not persisted: %+v", detail.Chunks)
	}
	if len(firstDetail.Items) != 1 || len(detail.Items) != 1 || firstDetail.Items[0].Version != detail.Items[0].Version {
		t.Fatalf("idempotent import changed source item version: before=%+v after=%+v", firstDetail.Items, detail.Items)
	}

	if _, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "", Content: "text"}); err == nil {
		t.Fatal("blank source name should be rejected")
	}
	if _, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "missing content"}); err == nil {
		t.Fatal("blank source content should be rejected")
	}

	badEmbedder := &testEmbeddingProvider{document: make([]float32, knowledge.EmbeddingDimensions-1)}
	app.SetKnowledgeEmbedder(badEmbedder)
	bad, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "No vector", URI: "memory://no-vector", Content: "Lexical fallback remains available."})
	if err != nil {
		t.Fatal(err)
	}
	badDetail, err := app.GetKnowledgeSourceDetail(ctx, bad.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(badDetail.Chunks) != 1 || len(badDetail.Chunks[0].Embedding) != 0 {
		t.Fatalf("invalid embedding should be discarded: %+v", badDetail.Chunks)
	}
}

func TestAppManualImportIdempotencyRejectsPayloadReuse(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	input := ManualSourceInput{
		Name:    "Idempotency runbook",
		Kind:    "runbook",
		URI:     "memory://idempotency-runbook",
		Content: "The confirmed rollback remains reversible.",
	}
	first, err := app.ImportManualSource(ctx, input, "manual-import-replay")
	if err != nil {
		t.Fatalf("first keyed manual import: %v", err)
	}
	replay, err := app.ImportManualSource(ctx, input, "manual-import-replay")
	if err != nil {
		t.Fatalf("same-payload keyed replay: %v", err)
	}
	if first != replay {
		t.Fatalf("keyed replay changed the canonical projection: first=%+v replay=%+v", first, replay)
	}

	input.Content = "The confirmed rollback was replaced."
	if _, err := app.ImportManualSource(ctx, input, "manual-import-replay"); !errors.Is(err, knowledge.ErrConflict) {
		t.Fatalf("same key with a different payload error = %v, want knowledge conflict", err)
	}
}

func TestAppManualImportConcurrentKeyCommitsOneSourceGraph(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	input := ManualSourceInput{
		Name:    "Concurrent runbook",
		Kind:    "runbook",
		URI:     "memory://concurrent-runbook",
		Content: "The atomic import must produce one canonical source graph.",
	}
	const callers = 12
	results := make(chan ImportedSource, callers)
	errorsCh := make(chan error, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, importErr := app.ImportManualSource(appContext(), input, "concurrent-manual-import")
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
		t.Fatalf("concurrent manual import: %v", importErr)
	}
	if len(results) != callers {
		t.Fatalf("completed imports = %d, want %d", len(results), callers)
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
			t.Fatalf("concurrent replay changed projection: first=%+v result=%+v", first, result)
		}
		index++
	}
	detail, err := app.GetKnowledgeSourceDetail(appContext(), first.Source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Items) != 1 || len(detail.Revisions) != 1 || len(detail.Chunks) != 1 {
		t.Fatalf("concurrent import created duplicate graph rows: items=%d revisions=%d chunks=%d", len(detail.Items), len(detail.Revisions), len(detail.Chunks))
	}
}

func TestAppHybridRetrievalFallsBackToDegradedLexicalMode(t *testing.T) {
	repository := &testHybridRepository{MemoryRepository: knowledge.NewMemoryRepository()}
	app, err := New(work.NewMemoryRepository(), repository, NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	if _, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "Hybrid source", URI: "memory://hybrid", Content: "Hybrid retrieval uses a local embedding sidecar."}); err != nil {
		t.Fatal(err)
	}
	embedder := &testEmbeddingProvider{query: vector384(1)}
	app.SetKnowledgeEmbedder(embedder)
	hits, err := app.SearchKnowledge(ctx, "embedding", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].RetrievalMode != "hybrid" || embedder.queryCalls != 1 {
		t.Fatalf("hybrid search was not selected: hits=%+v calls=%d", hits, embedder.queryCalls)
	}

	embedder.queryErr = errors.New("sidecar unavailable")
	hits, err = app.SearchKnowledge(ctx, "embedding", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].RetrievalMode != "degraded" {
		t.Fatalf("embedding outage should expose degraded lexical mode: %+v", hits)
	}

	embedder.queryErr = nil
	repository.hybridErr = errors.New("vector query failed")
	hits, err = app.SearchKnowledge(ctx, "embedding", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].RetrievalMode != "degraded" {
		t.Fatalf("hybrid query failure should expose degraded lexical mode: %+v", hits)
	}

	app.SetKnowledgeEmbedder(nil)
	hits, err = app.SearchKnowledge(ctx, "embedding", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].RetrievalMode != "lexical" {
		t.Fatalf("nil embedder should select lexical mode: %+v", hits)
	}
	if _, err := app.Search(ctx, assistant.RetrievalRequest{}); err == nil {
		t.Fatal("Search should reject an incomplete assistant scope")
	}
}

func TestAppSearchPreservesHybridRelevanceOrder(t *testing.T) {
	workspaceID := WorkspaceIDForUser("user-1")
	repository := &testHybridRepository{
		MemoryRepository: knowledge.NewMemoryRepository(),
		hybridResults: []knowledge.SearchResult{
			{
				Source:   knowledge.KnowledgeSource{ID: "source-z", WorkspaceID: workspaceID, Name: "First ranked source", URI: "memory://z"},
				Item:     knowledge.SourceItem{ID: "item-z", WorkspaceID: workspaceID, SourceID: "source-z", URI: "memory://z"},
				Revision: knowledge.SourceRevision{ID: "revision-z", WorkspaceID: workspaceID, SourceItemID: "item-z", SourceURI: "memory://z", Content: "first"},
				Chunk:    knowledge.KnowledgeChunk{ID: "chunk-z", WorkspaceID: workspaceID, RevisionID: "revision-z", Text: "first ranked result"},
			},
			{
				Source:   knowledge.KnowledgeSource{ID: "source-a", WorkspaceID: workspaceID, Name: "Second ranked source", URI: "memory://a"},
				Item:     knowledge.SourceItem{ID: "item-a", WorkspaceID: workspaceID, SourceID: "source-a", URI: "memory://a"},
				Revision: knowledge.SourceRevision{ID: "revision-a", WorkspaceID: workspaceID, SourceItemID: "item-a", SourceURI: "memory://a", Content: "second"},
				Chunk:    knowledge.KnowledgeChunk{ID: "chunk-a", WorkspaceID: workspaceID, RevisionID: "revision-a", Text: "second ranked result"},
			},
		},
	}
	app, err := New(work.NewMemoryRepository(), repository, NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	app.SetKnowledgeEmbedder(&testEmbeddingProvider{query: vector384(1)})

	hits, err := app.SearchKnowledge(appContext(), "ranked", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want two bounded hybrid hits: %+v", len(hits), hits)
	}
	wantIDs := []string{"knowledge-chunk-z", "knowledge-chunk-a"}
	for index, wantID := range wantIDs {
		if hits[index].ID != wantID || hits[index].RetrievalMode != "hybrid" {
			t.Fatalf("hybrid hit %d = %+v, want id %q with hybrid mode", index, hits[index], wantID)
		}
	}
}

func TestAppSearchRejectsScopeOutsideAuthenticatedContext(t *testing.T) {
	app, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	ctx := appContext()
	if _, err := app.ImportManualSource(ctx, ManualSourceInput{
		Name:    "Private source",
		URI:     "memory://private-source",
		Content: "This evidence belongs to the authenticated user.",
	}); err != nil {
		t.Fatal(err)
	}

	wrongUser := assistant.RetrievalRequest{
		Scope: assistant.Scope{
			WorkspaceID: WorkspaceIDForUser("user-2"),
			UserID:      "user-2",
		},
		Query: "private evidence",
	}
	if _, err := app.Search(ctx, wrongUser); err == nil {
		t.Fatal("Search should reject a scope belonging to another authenticated user")
	}

	wrongWorkspace := assistant.RetrievalRequest{
		Scope: assistant.Scope{
			WorkspaceID: "workspace-forged",
			UserID:      "user-1",
		},
		Query: "private evidence",
	}
	if _, err := app.Search(ctx, wrongWorkspace); err == nil {
		t.Fatal("Search should reject a forged workspace for the authenticated user")
	}
}

func TestApplicationHelpersAndDeterministicGenerator(t *testing.T) {
	if got := firstLine("  first line\nsecond line "); got != "first line" {
		t.Fatalf("firstLine = %q", got)
	}
	long := strings.Repeat("x", 140)
	if len(firstLine(long)) != 120 {
		t.Fatalf("firstLine should be bounded, got %d bytes", len(firstLine(long)))
	}
	if got := firstNonEmpty(" ", " value ", "other"); got != "value" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if got := searchTerms("API api x rollback"); !reflect.DeepEqual(got, []string{"api", "rollback"}) {
		t.Fatalf("searchTerms = %#v", got)
	}
	if !matchesTerms([]string{"rollback"}, "A rollback runbook") || matchesTerms(nil, "rollback") {
		t.Fatal("matchesTerms boundary is incorrect")
	}

	now := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	if revisionFreshness(knowledge.SourceRevision{IngestedAt: now.Add(-knowledgeStaleAfter)}, now) != "stale" {
		t.Fatal("old revisions should be stale")
	}
	if revisionFreshness(knowledge.SourceRevision{}, now) != "current" {
		t.Fatal("revision without a timestamp should remain current")
	}

	generator := deterministicGenerator{}
	if _, err := generator.Generate(context.Background(), assistant.GenerationRequest{}); err == nil {
		t.Fatal("deterministic generator should reject empty evidence")
	}
	draft, err := generator.Generate(context.Background(), assistant.GenerationRequest{Evidence: []assistant.Evidence{{ID: "evidence-1", Snippet: "canonical fact", URI: "memory://fact"}}})
	if err != nil || len(draft.Citations) != 1 || !strings.Contains(draft.Answer, "canonical fact") {
		t.Fatalf("deterministic draft = %+v err=%v", draft, err)
	}
	if usageRecordTokens(domain.UsageRecord{}) != 0 || usageRecordTokens(domain.UsageRecord{InputTokens: 2, OutputTokens: 3}) != 5 {
		t.Fatal("usageRecordTokens should use provider token counts")
	}
}
