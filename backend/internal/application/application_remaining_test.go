package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

// These boundary repositories deliberately fail one application dependency at
// a time. The application is the composition root, so its error propagation
// and fail-closed behavior deserve tests independent of repository internals.
type applicationWorkBoundaryRepository struct {
	*work.MemoryRepository
	createProjectErr  error
	createTaskErr     error
	createDecisionErr error
	getProjectErr     error
	getTaskErr        error
	getDecisionErr    error
	listProjectsErr   error
	listTasksErr      error
	listDecisionsErr  error
}

func (r *applicationWorkBoundaryRepository) CreateProject(ctx context.Context, scope work.Scope, project work.Project, mutation work.Mutation) (work.Project, error) {
	if r.createProjectErr != nil {
		return work.Project{}, r.createProjectErr
	}
	return r.MemoryRepository.CreateProject(ctx, scope, project, mutation)
}

func (r *applicationWorkBoundaryRepository) CreateTask(ctx context.Context, scope work.Scope, task work.Task, mutation work.Mutation) (work.Task, error) {
	if r.createTaskErr != nil {
		return work.Task{}, r.createTaskErr
	}
	return r.MemoryRepository.CreateTask(ctx, scope, task, mutation)
}

func (r *applicationWorkBoundaryRepository) CreateDecision(ctx context.Context, scope work.Scope, decision work.Decision, mutation work.Mutation) (work.Decision, error) {
	if r.createDecisionErr != nil {
		return work.Decision{}, r.createDecisionErr
	}
	return r.MemoryRepository.CreateDecision(ctx, scope, decision, mutation)
}

func (r *applicationWorkBoundaryRepository) GetProject(ctx context.Context, scope work.Scope, id string) (work.Project, error) {
	if r.getProjectErr != nil {
		return work.Project{}, r.getProjectErr
	}
	return r.MemoryRepository.GetProject(ctx, scope, id)
}

func (r *applicationWorkBoundaryRepository) GetTask(ctx context.Context, scope work.Scope, id string) (work.Task, error) {
	if r.getTaskErr != nil {
		return work.Task{}, r.getTaskErr
	}
	return r.MemoryRepository.GetTask(ctx, scope, id)
}

func (r *applicationWorkBoundaryRepository) GetDecision(ctx context.Context, scope work.Scope, id string) (work.Decision, error) {
	if r.getDecisionErr != nil {
		return work.Decision{}, r.getDecisionErr
	}
	return r.MemoryRepository.GetDecision(ctx, scope, id)
}

func (r *applicationWorkBoundaryRepository) ListProjects(ctx context.Context, scope work.Scope, options work.ListOptions) ([]work.Project, error) {
	if r.listProjectsErr != nil {
		return nil, r.listProjectsErr
	}
	return r.MemoryRepository.ListProjects(ctx, scope, options)
}

func (r *applicationWorkBoundaryRepository) ListTasks(ctx context.Context, scope work.Scope, projectID string, options work.ListOptions) ([]work.Task, error) {
	if r.listTasksErr != nil {
		return nil, r.listTasksErr
	}
	return r.MemoryRepository.ListTasks(ctx, scope, projectID, options)
}

func (r *applicationWorkBoundaryRepository) ListDecisions(ctx context.Context, scope work.Scope, projectID string, options work.ListOptions) ([]work.Decision, error) {
	if r.listDecisionsErr != nil {
		return nil, r.listDecisionsErr
	}
	return r.MemoryRepository.ListDecisions(ctx, scope, projectID, options)
}

func newApplicationWorkBoundaryRepository() *applicationWorkBoundaryRepository {
	return &applicationWorkBoundaryRepository{MemoryRepository: work.NewMemoryRepository()}
}

type applicationKnowledgeBoundaryRepository struct {
	*knowledge.MemoryRepository
	createSourceErr       error
	getSourceErr          error
	listSourcesErr        error
	createSourceItemErr   error
	createRevisionErr     error
	getRevisionErr        error
	createChunkErr        error
	setCurrentRevisionErr error
	searchErr             error
	listSourceItemsErr    error
	listRevisionsErr      error
	listChunksErr         error
	listEvidenceErr       error
	searchResults         []knowledge.SearchResult
	revision              knowledge.SourceRevision
}

func (r *applicationKnowledgeBoundaryRepository) CreateSource(ctx context.Context, scope knowledge.WorkspaceScope, source knowledge.KnowledgeSource) error {
	if r.createSourceErr != nil {
		return r.createSourceErr
	}
	return r.MemoryRepository.CreateSource(ctx, scope, source)
}

func (r *applicationKnowledgeBoundaryRepository) GetSource(ctx context.Context, scope knowledge.WorkspaceScope, id string) (knowledge.KnowledgeSource, error) {
	if r.getSourceErr != nil {
		return knowledge.KnowledgeSource{}, r.getSourceErr
	}
	return r.MemoryRepository.GetSource(ctx, scope, id)
}

func (r *applicationKnowledgeBoundaryRepository) ListSources(ctx context.Context, scope knowledge.WorkspaceScope, limit int) ([]knowledge.KnowledgeSource, error) {
	if r.listSourcesErr != nil {
		return nil, r.listSourcesErr
	}
	return r.MemoryRepository.ListSources(ctx, scope, limit)
}

func (r *applicationKnowledgeBoundaryRepository) CreateSourceItem(ctx context.Context, scope knowledge.WorkspaceScope, item knowledge.SourceItem) error {
	if r.createSourceItemErr != nil {
		return r.createSourceItemErr
	}
	return r.MemoryRepository.CreateSourceItem(ctx, scope, item)
}

func (r *applicationKnowledgeBoundaryRepository) CreateRevision(ctx context.Context, scope knowledge.WorkspaceScope, revision knowledge.SourceRevision) error {
	if r.createRevisionErr != nil {
		return r.createRevisionErr
	}
	return r.MemoryRepository.CreateRevision(ctx, scope, revision)
}

func (r *applicationKnowledgeBoundaryRepository) GetRevision(ctx context.Context, scope knowledge.WorkspaceScope, id string) (knowledge.SourceRevision, error) {
	if r.getRevisionErr != nil {
		return knowledge.SourceRevision{}, r.getRevisionErr
	}
	if r.revision.ID != "" {
		return r.revision, nil
	}
	return r.MemoryRepository.GetRevision(ctx, scope, id)
}

func (r *applicationKnowledgeBoundaryRepository) CreateChunk(ctx context.Context, scope knowledge.WorkspaceScope, chunk knowledge.KnowledgeChunk) error {
	if r.createChunkErr != nil {
		return r.createChunkErr
	}
	return r.MemoryRepository.CreateChunk(ctx, scope, chunk)
}

func (r *applicationKnowledgeBoundaryRepository) SetCurrentRevision(ctx context.Context, scope knowledge.WorkspaceScope, itemID, revisionID string) error {
	if r.setCurrentRevisionErr != nil {
		return r.setCurrentRevisionErr
	}
	return r.MemoryRepository.SetCurrentRevision(ctx, scope, itemID, revisionID)
}

func (r *applicationKnowledgeBoundaryRepository) Search(ctx context.Context, scope knowledge.WorkspaceScope, query string, limit int) ([]knowledge.SearchResult, error) {
	if r.searchErr != nil {
		return nil, r.searchErr
	}
	if r.searchResults != nil {
		return r.searchResults, nil
	}
	return r.MemoryRepository.Search(ctx, scope, query, limit)
}

func (r *applicationKnowledgeBoundaryRepository) ListSourceItems(ctx context.Context, scope knowledge.WorkspaceScope, sourceID string, limit int) ([]knowledge.SourceItem, error) {
	if r.listSourceItemsErr != nil {
		return nil, r.listSourceItemsErr
	}
	return r.MemoryRepository.ListSourceItems(ctx, scope, sourceID, limit)
}

func (r *applicationKnowledgeBoundaryRepository) ListRevisions(ctx context.Context, scope knowledge.WorkspaceScope, itemID string, limit int) ([]knowledge.SourceRevision, error) {
	if r.listRevisionsErr != nil {
		return nil, r.listRevisionsErr
	}
	return r.MemoryRepository.ListRevisions(ctx, scope, itemID, limit)
}

func (r *applicationKnowledgeBoundaryRepository) ListChunks(ctx context.Context, scope knowledge.WorkspaceScope, revisionID string, limit int) ([]knowledge.KnowledgeChunk, error) {
	if r.listChunksErr != nil {
		return nil, r.listChunksErr
	}
	return r.MemoryRepository.ListChunks(ctx, scope, revisionID, limit)
}

func (r *applicationKnowledgeBoundaryRepository) ListEvidenceForSource(ctx context.Context, scope knowledge.WorkspaceScope, sourceID string, limit int) ([]knowledge.ClaimEvidence, error) {
	if r.listEvidenceErr != nil {
		return nil, r.listEvidenceErr
	}
	return r.MemoryRepository.ListEvidenceForSource(ctx, scope, sourceID, limit)
}

func newApplicationKnowledgeBoundaryRepository() *applicationKnowledgeBoundaryRepository {
	return &applicationKnowledgeBoundaryRepository{MemoryRepository: knowledge.NewMemoryRepository()}
}

type applicationConversationBoundaryRepository struct {
	*MemoryConversationRepository
	createErr   error
	getErr      error
	listErr     error
	appendErr   error
	getOverride *Conversation
}

func (r *applicationConversationBoundaryRepository) Create(ctx context.Context, scope work.Scope, title string, contexts ...assistant.ContextRef) (Conversation, error) {
	if r.createErr != nil {
		return Conversation{}, r.createErr
	}
	return r.MemoryConversationRepository.Create(ctx, scope, title, contexts...)
}

func (r *applicationConversationBoundaryRepository) Get(ctx context.Context, scope work.Scope, id string) (Conversation, error) {
	if r.getErr != nil {
		return Conversation{}, r.getErr
	}
	if r.getOverride != nil {
		return *r.getOverride, nil
	}
	return r.MemoryConversationRepository.Get(ctx, scope, id)
}

func (r *applicationConversationBoundaryRepository) List(ctx context.Context, scope work.Scope, limit int) ([]ConversationSummary, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.MemoryConversationRepository.List(ctx, scope, limit)
}

func (r *applicationConversationBoundaryRepository) Append(ctx context.Context, scope work.Scope, id, message string, response assistant.AssistantResponse) error {
	if r.appendErr != nil {
		return r.appendErr
	}
	return r.MemoryConversationRepository.Append(ctx, scope, id, message, response)
}

func newApplicationConversationBoundaryRepository() *applicationConversationBoundaryRepository {
	return &applicationConversationBoundaryRepository{MemoryConversationRepository: NewMemoryConversationRepository()}
}

func TestBootstrapWithModulesSkipsDisabledRepositoriesAndKeepsArrays(t *testing.T) {
	readError := errors.New("disabled module must not be queried")
	workRepo := newApplicationWorkBoundaryRepository()
	workRepo.listProjectsErr = readError
	workRepo.listTasksErr = readError
	workRepo.listDecisionsErr = readError
	knowledgeRepo := newApplicationKnowledgeBoundaryRepository()
	knowledgeRepo.listSourcesErr = readError
	conversationRepo := newApplicationConversationBoundaryRepository()
	conversationRepo.getErr = readError
	conversationRepo.listErr = readError

	app, err := New(workRepo, knowledgeRepo, conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := app.BootstrapWithModules(appContext(), false, BootstrapModules{})
	if err != nil {
		t.Fatalf("disabled bootstrap queried an optional dependency: %v", err)
	}
	if bootstrap.Workspace.ID == "" || len(bootstrap.Projects) != 0 || len(bootstrap.Tasks) != 0 || len(bootstrap.Decisions) != 0 || len(bootstrap.Sources) != 0 || bootstrap.Conversation != nil || len(bootstrap.Conversations) != 0 {
		t.Fatalf("disabled bootstrap leaked module data: %+v", bootstrap)
	}

	payload, err := json.Marshal(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"projects", "tasks", "decisions", "sources", "conversations"} {
		if got := string(fields[field]); got != "[]" {
			t.Fatalf("bootstrap.%s = %s, want empty JSON array", field, got)
		}
	}
}

func TestBootstrapWithModulesCopiesCapabilities(t *testing.T) {
	app, err := New(
		work.NewMemoryRepository(),
		knowledge.NewMemoryRepository(),
		NewMemoryConversationRepository(),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := map[string]bool{"work": true, "knowledge": false}
	bootstrap, err := app.BootstrapWithModules(appContext(), false, BootstrapModules{
		Capabilities: capabilities,
	})
	if err != nil {
		t.Fatal(err)
	}
	capabilities["work"] = false
	capabilities["assistant"] = true
	if !bootstrap.Capabilities["work"] || bootstrap.Capabilities["knowledge"] || bootstrap.Capabilities["assistant"] {
		t.Fatalf("bootstrap capabilities were not defensively copied: %#v", bootstrap.Capabilities)
	}
	if bootstrap.Capabilities == nil {
		t.Fatal("bootstrap capabilities must be a non-nil JSON object")
	}
}

type applicationQueryOnlyEmbedder struct{}

func (applicationQueryOnlyEmbedder) Embed(context.Context, string) ([]float32, error) {
	return vector384(1), nil
}

func TestApplicationPropagatesCompositionAndRepositoryFailures(t *testing.T) {
	ctx := appContext()
	readError := errors.New("dependency read failed")

	for _, testCase := range []struct {
		name string
		repo *applicationWorkBoundaryRepository
		want string
	}{
		{name: "bootstrap projects", repo: func() *applicationWorkBoundaryRepository {
			r := newApplicationWorkBoundaryRepository()
			r.listProjectsErr = readError
			return r
		}(), want: "dependency read failed"},
		{name: "bootstrap tasks", repo: func() *applicationWorkBoundaryRepository {
			r := newApplicationWorkBoundaryRepository()
			r.listTasksErr = readError
			return r
		}(), want: "dependency read failed"},
		{name: "bootstrap decisions", repo: func() *applicationWorkBoundaryRepository {
			r := newApplicationWorkBoundaryRepository()
			r.listDecisionsErr = readError
			return r
		}(), want: "dependency read failed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			app, err := New(testCase.repo, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.Bootstrap(ctx); !errors.Is(err, readError) {
				t.Fatalf("Bootstrap error = %v, want %s", err, testCase.want)
			}
		})
	}

	knowledgeRepo := newApplicationKnowledgeBoundaryRepository()
	knowledgeRepo.listSourcesErr = readError
	app, err := New(newApplicationWorkBoundaryRepository(), knowledgeRepo, NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Bootstrap(ctx); !errors.Is(err, readError) {
		t.Fatalf("Bootstrap source-list error = %v", err)
	}

	conversationRepo := newApplicationConversationBoundaryRepository()
	conversationRepo.getErr = readError
	app, err = New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Bootstrap(ctx); !errors.Is(err, readError) {
		t.Fatalf("Bootstrap latest conversation error = %v", err)
	}

	conversationRepo = newApplicationConversationBoundaryRepository()
	conversationRepo.listErr = readError
	app, err = New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Bootstrap(ctx); !errors.Is(err, readError) {
		t.Fatalf("Bootstrap conversation-list error = %v", err)
	}

	for _, testCase := range []struct {
		name string
		call func(*App) error
	}{
		{name: "project create", call: func(app *App) error {
			_, err := app.CreateProject(ctx, work.CreateProjectInput{Name: "project"}, "project-error")
			return err
		}},
		{name: "task create", call: func(app *App) error {
			_, err := app.CreateTask(ctx, work.CreateTaskInput{Title: "task"}, "task-error")
			return err
		}},
		{name: "decision create", call: func(app *App) error {
			_, err := app.CreateDecision(ctx, work.CreateDecisionInput{Title: "decision", Outcome: "recorded outcome"}, "decision-error")
			return err
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newApplicationWorkBoundaryRepository()
			switch testCase.name {
			case "project create":
				repo.createProjectErr = readError
			case "task create":
				repo.createTaskErr = readError
			case "decision create":
				repo.createDecisionErr = readError
			}
			app, err := New(repo, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := testCase.call(app); !errors.Is(err, readError) {
				t.Fatalf("%s error = %v", testCase.name, err)
			}
		})
	}
}

func TestApplicationManualImportFailsClosedAtEveryPersistenceBoundary(t *testing.T) {
	ctx := appContext()
	input := ManualSourceInput{Name: "Boundary source", URI: "memory://boundary", Content: "Boundary content."}
	dependencyError := errors.New("boundary dependency failed")

	for _, testCase := range []struct {
		name      string
		configure func(*applicationKnowledgeBoundaryRepository)
	}{
		{name: "create source", configure: func(r *applicationKnowledgeBoundaryRepository) { r.createSourceErr = dependencyError }},
		{name: "existing source lookup", configure: func(r *applicationKnowledgeBoundaryRepository) {
			r.createSourceErr = knowledge.ErrConflict
			r.getSourceErr = dependencyError
		}},
		{name: "create source item", configure: func(r *applicationKnowledgeBoundaryRepository) { r.createSourceItemErr = dependencyError }},
		{name: "append revision", configure: func(r *applicationKnowledgeBoundaryRepository) { r.createRevisionErr = dependencyError }},
		{name: "existing revision lookup", configure: func(r *applicationKnowledgeBoundaryRepository) {
			r.createRevisionErr = knowledge.ErrConflict
			r.getRevisionErr = dependencyError
		}},
		{name: "create chunk", configure: func(r *applicationKnowledgeBoundaryRepository) { r.createChunkErr = dependencyError }},
		{name: "set current revision", configure: func(r *applicationKnowledgeBoundaryRepository) { r.setCurrentRevisionErr = dependencyError }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newApplicationKnowledgeBoundaryRepository()
			testCase.configure(repo)
			app, err := New(newApplicationWorkBoundaryRepository(), repo, NewMemoryConversationRepository(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.ImportManualSource(ctx, input); !errors.Is(err, dependencyError) {
				t.Fatalf("ImportManualSource error = %v, want dependency failure", err)
			}
		})
	}

	app, err := New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	app.SetKnowledgeEmbedder(applicationQueryOnlyEmbedder{})
	if _, err := app.ImportManualSource(ctx, input); err != nil {
		t.Fatalf("query-only embedder should keep import lexical-only: %v", err)
	}
	if sameManualRevision(knowledge.SourceRevision{ID: "a"}, knowledge.SourceRevision{ID: "b"}) {
		// Unequal revisions must never be accepted as an idempotent retry.
		t.Fatal("different revisions unexpectedly matched")
	}
}

func TestApplicationAskAndContextBoundaries(t *testing.T) {
	ctx := appContext()
	dependencyError := errors.New("conversation dependency failed")

	conversationRepo := newApplicationConversationBoundaryRepository()
	conversationRepo.createErr = dependencyError
	app, err := New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Ask(ctx, "", "hello"); !errors.Is(err, dependencyError) {
		t.Fatalf("conversation create error = %v", err)
	}

	conversationRepo = newApplicationConversationBoundaryRepository()
	conversationRepo.getErr = dependencyError
	app, err = New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Ask(ctx, "conversation-1", "hello"); !errors.Is(err, dependencyError) {
		t.Fatalf("conversation get error = %v", err)
	}

	conversationRepo = newApplicationConversationBoundaryRepository()
	conversationRepo.appendErr = dependencyError
	app, err = New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Ask(ctx, "", "hello"); !errors.Is(err, dependencyError) {
		t.Fatalf("conversation append error = %v", err)
	}

	app, err = New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateProject(ctx, work.CreateProjectInput{ID: "assistant-error-project", Name: "hello"}, "assistant-error-project"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetAssistantGenerator(testAssistantGenerator{err: dependencyError}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Ask(ctx, "", "hello"); err == nil {
		t.Fatalf("assistant generator error = %v", err)
	}

	invalidConversation := Conversation{
		ID: "conversation-invalid-context", WorkspaceID: WorkspaceIDForUser("user-1"), UserID: "user-1",
		Context: &assistant.ContextRef{Type: "not-a-context", ID: "entity"}, Messages: []ConversationMessage{},
	}
	conversationRepo = newApplicationConversationBoundaryRepository()
	conversationRepo.getOverride = &invalidConversation
	app, err = New(newApplicationWorkBoundaryRepository(), knowledge.NewMemoryRepository(), conversationRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Ask(ctx, invalidConversation.ID, "hello"); err == nil {
		t.Fatal("invalid stored conversation context unexpectedly succeeded")
	}

	// A context cannot be attached to an existing context-free conversation.
	conflictWorkRepo := newApplicationWorkBoundaryRepository()
	conflictApp, err := New(conflictWorkRepo, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conflictApp.CreateProject(ctx, work.CreateProjectInput{ID: "context-project", Name: "Context project"}, "context-project-create"); err != nil {
		t.Fatal(err)
	}
	conversationRepo = newApplicationConversationBoundaryRepository()
	conversation, err := conversationRepo.MemoryConversationRepository.Create(ctx, work.Scope{WorkspaceID: WorkspaceIDForUser("user-1"), UserID: "user-1"}, "context-free")
	if err != nil {
		t.Fatal(err)
	}
	conversationRepo.getOverride = &conversation
	conflictApp.conversations = conversationRepo
	if _, err := conflictApp.AskWithContext(ctx, conversation.ID, "attach context", assistant.ContextRef{Type: assistant.ContextProject, ID: "context-project"}); !errors.Is(err, ErrConversationContextConflict) {
		t.Fatalf("context conflict error = %v, want ErrConversationContextConflict", err)
	}

	if _, err := app.AskWithContext(ctx, "", "hello", assistant.ContextRef{Type: "unsupported", ID: "id"}); err == nil {
		t.Fatal("unsupported context type unexpectedly succeeded")
	}
}

func TestApplicationContextEvidenceAndSearchFailureModes(t *testing.T) {
	ctx := appContext()
	workRepo := newApplicationWorkBoundaryRepository()
	app, err := New(workRepo, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	project, err := app.CreateProject(ctx, work.CreateProjectInput{ID: "boundary-project", Name: "Boundary project"}, "boundary-project-create")
	if err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 8, 29, 5, 0, 0, 0, time.UTC)
	task, err := app.CreateTask(ctx, work.CreateTaskInput{ID: "boundary-task", ProjectID: project.ID, Title: "Boundary task", DueAt: &due}, "boundary-task-create")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := app.CreateDecision(ctx, work.CreateDecisionInput{ID: "boundary-decision", ProjectID: project.ID, Title: "Boundary decision", Outcome: "Boundary outcome"}, "boundary-decision-create")
	if err != nil {
		t.Fatal(err)
	}
	source, err := app.ImportManualSource(ctx, ManualSourceInput{Name: "Boundary source", Content: "A canonical boundary source."})
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name string
		ref  assistant.ContextRef
		want string
	}{
		{name: "project lookup", ref: assistant.ContextRef{Type: assistant.ContextProject, ID: project.ID}, want: "Project: Boundary project"},
		{name: "task due date", ref: assistant.ContextRef{Type: assistant.ContextTask, ID: task.ID}, want: due.Format(time.RFC3339)},
		{name: "decision defaults", ref: assistant.ContextRef{Type: assistant.ContextDecision, ID: decision.ID}, want: "No context recorded."},
		{name: "source", ref: assistant.ContextRef{Type: assistant.ContextSource, ID: source.Source.ID}, want: "A canonical boundary source."},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			evidence, err := app.resolveContextEvidence(ctx, knowledge.WorkspaceScope{ID: WorkspaceIDForUser("user-1")}, testCase.ref)
			if err != nil || !strings.Contains(evidence.Snippet, testCase.want) {
				t.Fatalf("context evidence = %+v, err=%v, want %q", evidence, err, testCase.want)
			}
		})
	}

	for _, testCase := range []struct {
		name string
		repo *applicationWorkBoundaryRepository
	}{
		{name: "project error", repo: func() *applicationWorkBoundaryRepository {
			r := newApplicationWorkBoundaryRepository()
			r.getProjectErr = errors.New("project read failed")
			return r
		}()},
		{name: "task error", repo: func() *applicationWorkBoundaryRepository {
			r := newApplicationWorkBoundaryRepository()
			r.getTaskErr = errors.New("task read failed")
			return r
		}()},
		{name: "decision error", repo: func() *applicationWorkBoundaryRepository {
			r := newApplicationWorkBoundaryRepository()
			r.getDecisionErr = errors.New("decision read failed")
			return r
		}()},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			failureApp, err := New(testCase.repo, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := failureApp.resolveContextEvidence(ctx, knowledge.WorkspaceScope{ID: WorkspaceIDForUser("user-1")}, assistant.ContextRef{Type: map[string]string{"project error": assistant.ContextProject, "task error": assistant.ContextTask, "decision error": assistant.ContextDecision}[testCase.name], ID: "id"}); err == nil {
				t.Fatalf("%s unexpectedly succeeded", testCase.name)
			}
		})
	}

	searchRepo := newApplicationKnowledgeBoundaryRepository()
	searchRepo.searchErr = errors.New("search failed")
	searchApp, err := New(newApplicationWorkBoundaryRepository(), searchRepo, NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := searchApp.SearchKnowledge(ctx, "query", 0); !errors.Is(err, searchRepo.searchErr) {
		t.Fatalf("knowledge search error = %v", err)
	}

	workSearchRepo := newApplicationWorkBoundaryRepository()
	workSearchRepo.listProjectsErr = errors.New("work search failed")
	workSearchApp, err := New(workSearchRepo, knowledge.NewMemoryRepository(), NewMemoryConversationRepository(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workSearchApp.SearchKnowledge(ctx, "query", 1); !errors.Is(err, workSearchRepo.listProjectsErr) {
		t.Fatalf("work search error = %v", err)
	}

	if got := searchHitFromEvidence(assistant.Evidence{ID: "unknown", Freshness: assistant.FreshUnknown}); got.Freshness != "unknown" {
		t.Fatalf("unknown evidence freshness = %q", got.Freshness)
	}
	if got := searchHitFromEvidence(assistant.Evidence{ID: "stale", Freshness: assistant.FreshStale}); got.Freshness != "stale" {
		t.Fatalf("stale evidence freshness = %q", got.Freshness)
	}
	if got := revisionFreshness(knowledge.SourceRevision{ModifiedAt: due}, time.Time{}); got != "current" {
		t.Fatalf("zero clock freshness = %q", got)
	}
	if firstNonEmpty(" ", "\t") != "" || len(searchTerms("a b")) != 0 || matchesTerms([]string{"missing"}, "value") {
		t.Fatal("search helper empty/miss boundaries are incorrect")
	}
}
