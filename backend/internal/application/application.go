package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

const (
	defaultWorkspaceName  = "DevEnglish Workspace"
	maxSearchResults      = 100
	manualImportOperation = "knowledge.import_manual"
)

// Workspace is the authenticated user's current workspace projection. V1
// intentionally resolves one private workspace per user; a workspace switcher
// can be added without changing the application service boundary later.
type Workspace struct {
	ID      string `json:"id"`
	UserID  string `json:"userId"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Version int64  `json:"version"`
}

type SourceSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	URI       string    `json:"uri"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SearchHit struct {
	ID            string `json:"id"`
	SourceID      string `json:"sourceId"`
	SourceName    string `json:"sourceName"`
	RevisionID    string `json:"revisionId"`
	URI           string `json:"uri"`
	Excerpt       string `json:"excerpt"`
	Freshness     string `json:"freshness"`
	Origin        string `json:"origin"`
	RetrievalMode string `json:"retrievalMode,omitempty"`
}

type Bootstrap struct {
	Workspace     Workspace             `json:"workspace"`
	Projects      []work.Project        `json:"projects"`
	Tasks         []work.Task           `json:"tasks"`
	Decisions     []work.Decision       `json:"decisions"`
	Sources       []SourceSummary       `json:"sources"`
	Conversation  *Conversation         `json:"conversation,omitempty"`
	Conversations []ConversationSummary `json:"conversations"`
	Capabilities  map[string]bool       `json:"capabilities"`
}

// BootstrapModules controls which optional module projections are loaded into
// the bootstrap response. It is intentionally expressed in the application
// package so transport adapters cannot accidentally couple the application to
// a particular manifest implementation.
type BootstrapModules struct {
	IncludeWork      bool
	IncludeKnowledge bool
	IncludeAssistant bool
	// Capabilities is supplied by the transport composition so the
	// application does not depend on a particular manifest implementation.
	// BootstrapWithModules copies it before returning to prevent callers from
	// mutating the response after construction.
	Capabilities map[string]bool
}

type Conversation struct {
	ID          string                `json:"id"`
	WorkspaceID string                `json:"workspaceId"`
	UserID      string                `json:"userId"`
	Title       string                `json:"title"`
	Status      string                `json:"status"`
	Context     *assistant.ContextRef `json:"context,omitempty"`
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
	Messages    []ConversationMessage `json:"messages"`
}

// ConversationSummary is the list projection for the assistant history. It
// deliberately omits message bodies so a history screen cannot accidentally
// load an unbounded transcript when it only needs navigation metadata.
type ConversationSummary struct {
	ID           string                `json:"id"`
	WorkspaceID  string                `json:"workspaceId"`
	UserID       string                `json:"userId"`
	Title        string                `json:"title"`
	Status       string                `json:"status"`
	Context      *assistant.ContextRef `json:"context,omitempty"`
	MessageCount int                   `json:"messageCount"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

type ConversationMessage struct {
	ID        string                       `json:"id"`
	Role      string                       `json:"role"`
	Content   string                       `json:"content"`
	Response  *assistant.AssistantResponse `json:"response,omitempty"`
	CreatedAt time.Time                    `json:"createdAt"`
}

type AskResult struct {
	ConversationID string                      `json:"conversationId"`
	Response       assistant.AssistantResponse `json:"response"`
}

type ManualSourceInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType"`
	Content  string `json:"content"`
}

type ImportedSource struct {
	Source     SourceSummary `json:"source"`
	RevisionID string        `json:"revisionId"`
	ChunkID    string        `json:"chunkId"`
}

type ConversationRepository interface {
	Create(context.Context, work.Scope, string, ...assistant.ContextRef) (Conversation, error)
	List(context.Context, work.Scope, int) ([]ConversationSummary, error)
	Get(context.Context, work.Scope, string) (Conversation, error)
	Append(context.Context, work.Scope, string, string, assistant.AssistantResponse) error
}

type workspaceEnsurer interface {
	EnsureWorkspace(context.Context, string, string) error
}

// App is the composition root for the product surface. REST and MCP adapters
// should call this object or the bounded services it owns; they must not build
// a second set of business rules.
type App struct {
	Work              *work.Service
	Knowledge         *knowledge.Service
	Assistant         *assistant.Service
	workRepository    work.Repository
	knowledgeRepo     knowledge.Repository
	knowledgeSearcher knowledge.Searcher
	knowledgeEmbedder knowledge.EmbeddingProvider
	sources           knowledge.SourceLister
	conversations     ConversationRepository
	ensureWorkspace   workspaceEnsurer
}

func New(workRepository work.Repository, knowledgeRepository knowledge.Repository, conversations ConversationRepository, ensure workspaceEnsurer) (*App, error) {
	if workRepository == nil || knowledgeRepository == nil || conversations == nil {
		return nil, errors.New("application dependencies are incomplete")
	}
	workService, err := work.NewService(workRepository)
	if err != nil {
		return nil, fmt.Errorf("create work service: %w", err)
	}
	knowledgeService, err := knowledge.NewService(knowledgeRepository)
	if err != nil {
		return nil, fmt.Errorf("create knowledge service: %w", err)
	}
	searcher, _ := knowledgeRepository.(knowledge.Searcher)
	sources, _ := knowledgeRepository.(knowledge.SourceLister)
	app := &App{
		Work:              workService,
		Knowledge:         knowledgeService,
		workRepository:    workRepository,
		knowledgeRepo:     knowledgeRepository,
		knowledgeSearcher: searcher,
		sources:           sources,
		conversations:     conversations,
		ensureWorkspace:   ensure,
	}
	assistantService, err := assistant.NewService(app, deterministicGenerator{})
	if err != nil {
		return nil, fmt.Errorf("create assistant service: %w", err)
	}
	app.Assistant = assistantService
	return app, nil
}

// SetAssistantGenerator replaces only the provider implementation behind the
// canonical assistant service. REST, MCP and future adapters keep using the
// same retriever and response-normalization boundary.
func (a *App) SetAssistantGenerator(generator assistant.Generator) error {
	if a == nil || generator == nil {
		return errors.New("assistant generator is required")
	}
	service, err := assistant.NewService(a, generator)
	if err != nil {
		return err
	}
	a.Assistant = service
	return nil
}

// SetKnowledgeEmbedder enables semantic retrieval when the local sidecar is
// available. A nil embedder deliberately keeps lexical search as the default.
func (a *App) SetKnowledgeEmbedder(embedder knowledge.EmbeddingProvider) {
	if a != nil {
		a.knowledgeEmbedder = embedder
	}
}

func (a *App) Scope(ctx context.Context) (work.Scope, knowledge.WorkspaceScope, Workspace, error) {
	if ctx == nil {
		return work.Scope{}, knowledge.WorkspaceScope{}, Workspace{}, errors.New("context is required")
	}
	userID := store.UserID(ctx)
	workspaceID := WorkspaceIDForUser(userID)
	if a.ensureWorkspace != nil {
		if err := a.ensureWorkspace.EnsureWorkspace(ctx, workspaceID, userID); err != nil {
			return work.Scope{}, knowledge.WorkspaceScope{}, Workspace{}, err
		}
	}
	workScope := work.Scope{WorkspaceID: workspaceID, UserID: userID}
	knowledgeScope := knowledge.WorkspaceScope{ID: workspaceID}
	workspace := Workspace{ID: workspaceID, UserID: userID, Name: defaultWorkspaceName, Slug: workspaceID, Version: 1}
	return workScope, knowledgeScope, workspace, nil
}

func WorkspaceIDForUser(userID string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(userID)))
	return "workspace-" + hex.EncodeToString(hash[:8])
}

func (a *App) Bootstrap(ctx context.Context) (Bootstrap, error) {
	return a.BootstrapWithOptions(ctx, false)
}

func (a *App) BootstrapWithOptions(ctx context.Context, includeTrashed bool) (Bootstrap, error) {
	return a.BootstrapWithModules(ctx, includeTrashed, BootstrapModules{
		IncludeWork:      true,
		IncludeKnowledge: true,
		IncludeAssistant: true,
	})
}

// BootstrapWithModules returns a stable bootstrap envelope while loading only
// the selected module projections. Disabled modules stay empty and, most
// importantly, their repositories are not queried. The all-enabled wrapper
// above preserves the historical application behavior for existing callers.
func (a *App) BootstrapWithModules(ctx context.Context, includeTrashed bool, modules BootstrapModules) (Bootstrap, error) {
	workScope, knowledgeScope, workspace, err := a.Scope(ctx)
	if err != nil {
		return Bootstrap{}, err
	}
	projects := make([]work.Project, 0)
	tasks := make([]work.Task, 0)
	decisions := make([]work.Decision, 0)
	if modules.IncludeWork {
		projects, err = a.Work.ListProjects(ctx, workScope, work.ListOptions{Limit: 200, IncludeTrashed: includeTrashed})
		if err != nil {
			return Bootstrap{}, err
		}
		tasks, err = a.Work.ListTasks(ctx, workScope, "", work.ListOptions{Limit: 200, IncludeTrashed: includeTrashed})
		if err != nil {
			return Bootstrap{}, err
		}
		decisions, err = a.Work.ListDecisions(ctx, workScope, "", work.ListOptions{Limit: 200, IncludeTrashed: includeTrashed})
		if err != nil {
			return Bootstrap{}, err
		}
	}
	sources := make([]SourceSummary, 0)
	if modules.IncludeKnowledge && a.sources != nil {
		items, err := a.sources.ListSources(ctx, knowledgeScope, 200)
		if err != nil {
			return Bootstrap{}, err
		}
		for _, source := range items {
			sources = append(sources, sourceSummary(source))
		}
	}
	var conversation *Conversation
	conversations := make([]ConversationSummary, 0)
	if modules.IncludeAssistant {
		if item, err := a.conversations.Get(ctx, workScope, "latest"); err == nil {
			conversation = &item
		} else if !errors.Is(err, ErrConversationNotFound) {
			return Bootstrap{}, err
		}
		conversations, err = a.conversations.List(ctx, workScope, 20)
		if err != nil {
			return Bootstrap{}, err
		}
	}
	return Bootstrap{
		Workspace:     workspace,
		Projects:      projects,
		Tasks:         tasks,
		Decisions:     decisions,
		Sources:       sources,
		Conversation:  conversation,
		Conversations: conversations,
		Capabilities:  cloneCapabilities(modules.Capabilities),
	}, nil
}

func cloneCapabilities(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for name, enabled := range source {
		result[name] = enabled
	}
	return result
}

// ListConversations returns only the authenticated user's conversation
// metadata. Full transcripts remain behind GetConversation so callers opt in
// to loading message bodies explicitly.
func (a *App) ListConversations(ctx context.Context, limit int) ([]ConversationSummary, error) {
	workScope, _, _, err := a.Scope(ctx)
	if err != nil {
		return nil, err
	}
	return a.conversations.List(ctx, workScope, limit)
}

// GetConversation loads one transcript in the authenticated workspace.
func (a *App) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	workScope, _, _, err := a.Scope(ctx)
	if err != nil {
		return Conversation{}, err
	}
	return a.conversations.Get(ctx, workScope, strings.TrimSpace(conversationID))
}

func (a *App) CreateProject(ctx context.Context, input work.CreateProjectInput, idempotencyKey string) (work.Project, error) {
	workScope, _, _, err := a.Scope(ctx)
	if err != nil {
		return work.Project{}, err
	}
	return a.Work.CreateProject(ctx, workScope, input, idempotencyKey)
}

func (a *App) CreateTask(ctx context.Context, input work.CreateTaskInput, idempotencyKey string) (work.Task, error) {
	workScope, _, _, err := a.Scope(ctx)
	if err != nil {
		return work.Task{}, err
	}
	return a.Work.CreateTask(ctx, workScope, input, idempotencyKey)
}

// CreateDecision keeps decision creation on the same canonical application
// scope as project and task creation. Transport adapters should use this
// wrapper instead of reconstructing a workspace from request data.
func (a *App) CreateDecision(ctx context.Context, input work.CreateDecisionInput, idempotencyKey string) (work.Decision, error) {
	workScope, _, _, err := a.Scope(ctx)
	if err != nil {
		return work.Decision{}, err
	}
	return a.Work.CreateDecision(ctx, workScope, input, idempotencyKey)
}

func (a *App) ImportManualSource(ctx context.Context, input ManualSourceInput, idempotencyKeys ...string) (ImportedSource, error) {
	if len(idempotencyKeys) > 1 {
		return ImportedSource{}, errors.New("at most one idempotency key is supported")
	}
	key := ""
	if len(idempotencyKeys) == 1 {
		key = strings.TrimSpace(idempotencyKeys[0])
		if key == "" {
			return ImportedSource{}, errors.New("idempotency key must not be blank")
		}
	}
	workScope, knowledgeScope, _, err := a.Scope(ctx)
	if err != nil {
		return ImportedSource{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Kind = strings.TrimSpace(input.Kind)
	input.URI = strings.TrimSpace(input.URI)
	input.MIMEType = strings.TrimSpace(input.MIMEType)
	input.Content = strings.TrimSpace(input.Content)
	if input.Name == "" || input.Content == "" {
		return ImportedSource{}, errors.New("source name and content are required")
	}
	if input.Kind == "" {
		input.Kind = "manual"
	}
	requestHash := manualImportRequestHash(input)
	var idempotencyRepository knowledge.ManualImportIdempotencyRepository
	var atomicRepository knowledge.ManualImportRepository
	if key != "" {
		var ok bool
		atomicRepository, ok = a.knowledgeRepo.(knowledge.ManualImportRepository)
		if !ok {
			idempotencyRepository, ok = a.knowledgeRepo.(knowledge.ManualImportIdempotencyRepository)
			if !ok {
				return ImportedSource{}, errors.New("knowledge repository cannot persist manual import idempotency")
			}
			record, found, lookupErr := idempotencyRepository.LookupManualImportIdempotency(ctx, knowledgeScope, workScope.UserID, key)
			if lookupErr != nil {
				return ImportedSource{}, lookupErr
			}
			if found {
				return replayManualImport(record, requestHash)
			}
		}
	}
	contentHash := sha256.Sum256([]byte(input.Content))
	nameHash := sha256.Sum256([]byte(knowledgeScope.ID + "\x00" + input.Name + "\x00" + input.Kind + "\x00" + input.URI))
	sourceID := input.ID
	if sourceID == "" {
		sourceID = "manual-source-" + hex.EncodeToString(nameHash[:8])
	}
	itemHash := sha256.Sum256([]byte(knowledgeScope.ID + "\x00" + sourceID + "\x00" + input.Name))
	itemID := "manual-item-" + hex.EncodeToString(itemHash[:8])
	// Content hashes remain the canonical change detector, but the persisted
	// IDs also include the source item. Otherwise identical text imported into
	// two different sources would collide in the workspace-wide tables.
	revisionHash := sha256.Sum256([]byte(itemID + "\x00" + hex.EncodeToString(contentHash[:])))
	revisionID := "manual-revision-" + hex.EncodeToString(revisionHash[:8])
	chunkID := "manual-chunk-" + hex.EncodeToString(revisionHash[:8])
	now := time.Now().UTC()
	source := knowledge.KnowledgeSource{ID: sourceID, WorkspaceID: knowledgeScope.ID, Kind: input.Kind, Name: input.Name, URI: input.URI, Metadata: map[string]any{"origin": "manual"}, Version: 1, CreatedAt: now, UpdatedAt: now}
	if atomicRepository == nil {
		if err := a.Knowledge.CreateSource(ctx, knowledgeScope, source); err != nil {
			if !errors.Is(err, knowledge.ErrConflict) {
				return ImportedSource{}, err
			}
			source, err = a.knowledgeRepo.GetSource(ctx, knowledgeScope, sourceID)
			if err != nil {
				return ImportedSource{}, err
			}
		}
	}
	item := knowledge.SourceItem{ID: itemID, WorkspaceID: knowledgeScope.ID, SourceID: source.ID, ExternalID: source.ID, Title: input.Name, URI: input.URI, MIMEType: input.MIMEType, Version: 1, CreatedAt: now, UpdatedAt: now}
	if atomicRepository == nil {
		if err := a.Knowledge.CreateSourceItem(ctx, knowledgeScope, item); err != nil && !errors.Is(err, knowledge.ErrConflict) {
			return ImportedSource{}, err
		}
	}
	revision := knowledge.SourceRevision{ID: revisionID, WorkspaceID: knowledgeScope.ID, SourceItemID: itemID, RevisionKey: hex.EncodeToString(contentHash[:]), ContentHash: hex.EncodeToString(contentHash[:]), ContentType: input.MIMEType, SourceURI: input.URI, Content: input.Content, IngestedAt: now}
	if atomicRepository == nil {
		if err := a.Knowledge.AppendRevision(ctx, knowledgeScope, revision); err != nil {
			if !errors.Is(err, knowledge.ErrConflict) && !errors.Is(err, knowledge.ErrRevisionImmutable) {
				return ImportedSource{}, err
			}
			// The revision ID is deterministic, so a retry may race with or
			// encounter an existing immutable row. Timestamps are ingestion
			// metadata and are intentionally not part of the retry identity.
			existing, getErr := a.Knowledge.GetRevision(ctx, knowledgeScope, revisionID)
			if getErr != nil {
				return ImportedSource{}, getErr
			}
			if !sameManualRevision(existing, revision) {
				return ImportedSource{}, knowledge.ErrRevisionImmutable
			}
		}
	}
	chunk := knowledge.KnowledgeChunk{ID: chunkID, WorkspaceID: knowledgeScope.ID, RevisionID: revisionID, Ordinal: 0, Text: input.Content, TokenCount: len(strings.Fields(input.Content)), Embedding: a.embedKnowledgeDocument(ctx, input.Content), CreatedAt: now}
	if atomicRepository != nil {
		commit, commitErr := atomicRepository.CommitManualImport(ctx, knowledgeScope, knowledge.ManualImportCommitRequest{
			UserID:         workScope.UserID,
			IdempotencyKey: key,
			RequestHash:    requestHash,
			Bundle: knowledge.ManualImportBundle{
				Source: source, Item: item, Revision: revision, Chunk: chunk,
			},
		})
		if commitErr != nil {
			return ImportedSource{}, commitErr
		}
		if commit.Replayed {
			return replayManualImport(commit.Record, requestHash)
		}
		return importedSourceFromProjection(commit.Projection), nil
	}
	if err := a.Knowledge.CreateChunk(ctx, knowledgeScope, chunk); err != nil && !errors.Is(err, knowledge.ErrConflict) {
		return ImportedSource{}, err
	}
	setter, ok := a.knowledgeRepo.(knowledge.CurrentRevisionSetter)
	if !ok {
		return ImportedSource{}, errors.New("knowledge repository cannot set current revision")
	}
	if err := setter.SetCurrentRevision(ctx, knowledgeScope, itemID, revisionID); err != nil {
		return ImportedSource{}, err
	}
	result := ImportedSource{Source: sourceSummary(source), RevisionID: revisionID, ChunkID: chunkID}
	if idempotencyRepository != nil {
		responseJSON, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return ImportedSource{}, fmt.Errorf("encode manual import replay: %w", marshalErr)
		}
		record := knowledge.ManualImportIdempotencyRecord{
			WorkspaceID: knowledgeScope.ID, UserID: workScope.UserID,
			Operation: manualImportOperation, IdempotencyKey: key,
			RequestHash: requestHash, ResponseJSON: responseJSON,
		}
		if saveErr := idempotencyRepository.SaveManualImportIdempotency(ctx, knowledgeScope, record); saveErr != nil {
			if !errors.Is(saveErr, knowledge.ErrConflict) {
				return ImportedSource{}, saveErr
			}
			winner, found, lookupErr := idempotencyRepository.LookupManualImportIdempotency(ctx, knowledgeScope, workScope.UserID, key)
			if lookupErr != nil {
				return ImportedSource{}, lookupErr
			}
			if !found {
				return ImportedSource{}, saveErr
			}
			return replayManualImport(winner, requestHash)
		}
	}
	return result, nil
}

func manualImportRequestHash(input ManualSourceInput) string {
	payload, _ := json.Marshal(struct {
		Operation string            `json:"operation"`
		Input     ManualSourceInput `json:"input"`
	}{Operation: manualImportOperation, Input: input})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func replayManualImport(record knowledge.ManualImportIdempotencyRecord, requestHash string) (ImportedSource, error) {
	if record.Operation != manualImportOperation || record.RequestHash != requestHash {
		return ImportedSource{}, fmt.Errorf("%w: manual import idempotency key cannot be reused for a different request", knowledge.ErrConflict)
	}
	var result ImportedSource
	if err := json.Unmarshal(record.ResponseJSON, &result); err != nil {
		return ImportedSource{}, fmt.Errorf("decode manual import replay: %w", err)
	}
	return result, nil
}

func sameManualRevision(existing, expected knowledge.SourceRevision) bool {
	return existing.ID == expected.ID &&
		existing.WorkspaceID == expected.WorkspaceID &&
		existing.SourceItemID == expected.SourceItemID &&
		existing.RevisionKey == expected.RevisionKey &&
		existing.ContentHash == expected.ContentHash &&
		existing.ContentType == expected.ContentType &&
		existing.SourceURI == expected.SourceURI &&
		existing.Content == expected.Content
}

// embedKnowledgeDocument is deliberately best-effort. The sidecar is an
// acceleration path for hybrid retrieval; a sidecar outage must not discard a
// valid immutable source revision because lexical search remains available.
func (a *App) embedKnowledgeDocument(ctx context.Context, content string) []float32 {
	if a == nil || a.knowledgeEmbedder == nil {
		return nil
	}
	provider, ok := a.knowledgeEmbedder.(knowledge.DocumentEmbeddingProvider)
	if !ok {
		return nil
	}
	vector, err := provider.EmbedDocument(ctx, content)
	if err != nil || len(vector) != knowledge.EmbeddingDimensions {
		return nil
	}
	return vector
}

func (a *App) SearchKnowledge(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	_, knowledgeScope, _, err := a.Scope(ctx)
	if err != nil {
		return nil, err
	}
	return a.searchHits(ctx, knowledgeScope, query, limit)
}

func (a *App) GetKnowledgeSourceDetail(ctx context.Context, sourceID string) (knowledge.SourceDetail, error) {
	_, knowledgeScope, _, err := a.Scope(ctx)
	if err != nil {
		return knowledge.SourceDetail{}, err
	}
	return a.Knowledge.GetSourceDetail(ctx, knowledgeScope, sourceID)
}

func (a *App) Ask(ctx context.Context, conversationID, message string) (AskResult, error) {
	return a.AskWithContext(ctx, conversationID, message, assistant.ContextRef{})
}

// AskWithContext starts or continues a conversation while pinning it to one
// canonical entity. A conversation cannot silently change entity context;
// callers must start a new conversation when the focus changes.
func (a *App) AskWithContext(ctx context.Context, conversationID, message string, contextRef assistant.ContextRef) (AskResult, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return AskResult{}, fmt.Errorf("%w: message is required", ErrInvalidInput)
	}
	contextRef, err := contextRef.Normalize()
	if err != nil {
		return AskResult{}, err
	}
	workScope, knowledgeScope, _, err := a.Scope(ctx)
	if err != nil {
		return AskResult{}, err
	}
	if !contextRef.IsZero() {
		if _, err := a.resolveContextEvidence(ctx, knowledgeScope, contextRef); err != nil {
			return AskResult{}, err
		}
	}
	conversationID = strings.TrimSpace(conversationID)
	var conversation Conversation
	if conversationID == "" {
		if contextRef.IsZero() {
			conversation, err = a.conversations.Create(ctx, workScope, firstLine(message))
		} else {
			conversation, err = a.conversations.Create(ctx, workScope, firstLine(message), contextRef)
		}
		if err != nil {
			return AskResult{}, err
		}
		conversationID = conversation.ID
	} else {
		conversation, err = a.conversations.Get(ctx, workScope, conversationID)
		if err != nil {
			return AskResult{}, err
		}
		if conversation.Context != nil {
			storedContext, normalizeErr := conversation.Context.Normalize()
			if normalizeErr != nil {
				return AskResult{}, normalizeErr
			}
			if contextRef.IsZero() {
				contextRef = storedContext
			} else if storedContext != contextRef {
				return AskResult{}, ErrConversationContextConflict
			}
		} else if !contextRef.IsZero() {
			return AskResult{}, ErrConversationContextConflict
		}
	}
	history := make([]assistant.ConversationTurn, 0, len(conversation.Messages))
	for _, item := range conversation.Messages {
		history = append(history, assistant.ConversationTurn{Role: item.Role, Content: item.Content})
	}
	response, err := a.Assistant.Ask(ctx, assistant.AskRequest{Scope: assistant.Scope{WorkspaceID: workScope.WorkspaceID, UserID: workScope.UserID}, ConversationID: conversationID, Message: message, History: history, Context: contextRef})
	if err != nil {
		return AskResult{}, err
	}
	if err := a.conversations.Append(ctx, workScope, conversationID, message, response); err != nil {
		return AskResult{}, err
	}
	return AskResult{ConversationID: conversationID, Response: response}, nil
}

func (a *App) searchHits(ctx context.Context, scope knowledge.WorkspaceScope, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 || limit > maxSearchResults {
		limit = 20
	}
	result := make([]SearchHit, 0)
	if a.knowledgeSearcher != nil {
		items, retrievalMode, err := a.searchKnowledge(ctx, scope, query, limit)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			result = append(result, SearchHit{ID: "knowledge-" + item.Chunk.ID, SourceID: item.Source.ID, SourceName: item.Source.Name, RevisionID: item.Revision.ID, URI: firstNonEmpty(item.Revision.SourceURI, item.Source.URI, item.Item.URI), Excerpt: item.Chunk.Text, Freshness: revisionFreshness(item.Revision, a.now()), Origin: "canonical", RetrievalMode: retrievalMode})
		}
	}
	workScope := work.Scope{WorkspaceID: scope.ID, UserID: store.UserID(ctx)}
	projects, err := a.Work.ListProjects(ctx, workScope, work.ListOptions{Limit: 200})
	if err != nil {
		return nil, err
	}
	tasks, err := a.Work.ListTasks(ctx, workScope, "", work.ListOptions{Limit: 200})
	if err != nil {
		return nil, err
	}
	decisions, err := a.Work.ListDecisions(ctx, workScope, "", work.ListOptions{Limit: 200})
	if err != nil {
		return nil, err
	}
	terms := searchTerms(query)
	for _, project := range projects {
		if !matchesTerms(terms, project.Name+" "+project.Description) {
			continue
		}
		result = append(result, SearchHit{ID: "work-project-" + project.ID, SourceID: "project:" + project.ID, SourceName: project.Name, Excerpt: firstNonEmpty(project.Description, project.Name), Freshness: "current", Origin: "canonical"})
	}
	for _, task := range tasks {
		if !matchesTerms(terms, task.Title+" "+task.Description) {
			continue
		}
		result = append(result, SearchHit{ID: "work-task-" + task.ID, SourceID: "task:" + task.ID, SourceName: task.Title, Excerpt: firstNonEmpty(task.Description, task.Title), Freshness: "current", Origin: "canonical"})
	}
	for _, decision := range decisions {
		decisionText := strings.Join([]string{decision.Title, decision.Context, decision.Outcome, decision.Rationale}, " ")
		if !matchesTerms(terms, decisionText) {
			continue
		}
		result = append(result, SearchHit{
			ID:         "work-decision-" + decision.ID,
			SourceID:   "decision:" + decision.ID,
			SourceName: decision.Title,
			Excerpt:    firstNonEmpty(decision.Outcome, firstNonEmpty(decision.Context, decision.Title)),
			Freshness:  "current",
			Origin:     "canonical",
		})
	}
	// Knowledge searchers own relevance ordering: PostgreSQL returns hybrid
	// RRF rank and the memory repository returns lexical score order. Sorting
	// by presentation ID here would destroy that ranking and could evict the
	// most relevant result when the combined limit is applied. Work lists are
	// already returned in their repository-defined stable order, so preserve
	// both module boundaries as they cross the application service.
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (a *App) searchKnowledge(ctx context.Context, scope knowledge.WorkspaceScope, query string, limit int) ([]knowledge.SearchResult, string, error) {
	if a.knowledgeEmbedder != nil {
		if hybrid, ok := a.knowledgeSearcher.(knowledge.HybridSearcher); ok {
			vector, err := a.knowledgeEmbedder.Embed(ctx, query)
			if err == nil {
				items, hybridErr := hybrid.SearchHybrid(ctx, scope, query, vector, limit)
				if hybridErr == nil {
					return items, "hybrid", nil
				}
			}
			items, lexicalErr := a.knowledgeSearcher.Search(ctx, scope, query, limit)
			return items, "degraded", lexicalErr
		}
	}
	items, err := a.knowledgeSearcher.Search(ctx, scope, query, limit)
	return items, "lexical", err
}

func (a *App) Search(ctx context.Context, request assistant.RetrievalRequest) ([]assistant.Evidence, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if request.Scope.WorkspaceID == "" || request.Scope.UserID == "" {
		return nil, errors.New("assistant scope is required")
	}
	userID := store.UserID(ctx)
	authenticatedScope := assistant.Scope{UserID: userID, WorkspaceID: WorkspaceIDForUser(userID)}
	if request.Scope != authenticatedScope {
		return nil, errors.New("assistant scope does not match authenticated context")
	}
	normalizedContext, err := request.Context.Normalize()
	if err != nil {
		return nil, err
	}
	request.Context = normalizedContext
	hits, err := a.searchHits(ctx, knowledge.WorkspaceScope{ID: authenticatedScope.WorkspaceID}, request.Query, 20)
	if err != nil {
		return nil, err
	}
	if !request.Context.IsZero() {
		contextEvidence, contextErr := a.resolveContextEvidence(ctx, knowledge.WorkspaceScope{ID: authenticatedScope.WorkspaceID}, request.Context)
		if contextErr != nil {
			return nil, contextErr
		}
		contextHit := searchHitFromEvidence(contextEvidence)
		ordered := make([]SearchHit, 0, len(hits)+1)
		ordered = append(ordered, contextHit)
		seen := map[string]struct{}{contextHit.ID: {}}
		for _, hit := range hits {
			if _, exists := seen[hit.ID]; exists {
				continue
			}
			seen[hit.ID] = struct{}{}
			ordered = append(ordered, hit)
		}
		hits = ordered
		if len(hits) > 20 {
			hits = hits[:20]
		}
	}
	result := make([]assistant.Evidence, 0, len(hits))
	for _, hit := range hits {
		freshness := assistant.FreshCurrent
		if hit.Freshness == "stale" {
			freshness = assistant.FreshStale
		} else if hit.Freshness != "current" {
			freshness = assistant.FreshUnknown
		}
		result = append(result, assistant.Evidence{Scope: authenticatedScope, ID: hit.ID, SourceID: hit.SourceID, RevisionID: hit.RevisionID, Title: hit.SourceName, URI: hit.URI, Snippet: hit.Excerpt, Freshness: freshness})
	}
	return result, nil
}

func (a *App) resolveContextEvidence(ctx context.Context, scope knowledge.WorkspaceScope, contextRef assistant.ContextRef) (assistant.Evidence, error) {
	workScope := work.Scope{WorkspaceID: scope.ID, UserID: store.UserID(ctx)}
	switch contextRef.Type {
	case assistant.ContextProject:
		project, err := a.Work.GetProject(ctx, workScope, contextRef.ID)
		if err != nil {
			return assistant.Evidence{}, err
		}
		return assistant.Evidence{
			Scope:     assistant.Scope{WorkspaceID: scope.ID, UserID: workScope.UserID},
			ID:        "work-project-" + project.ID,
			SourceID:  "project:" + project.ID,
			Title:     project.Name,
			URI:       "work://project/" + project.ID,
			Snippet:   boundEvidenceText(strings.Join([]string{"Project: " + project.Name, "Status: " + string(project.Status), "Description: " + firstNonEmpty(project.Description, "No description recorded.")}, "\n")),
			Freshness: assistant.FreshCurrent,
		}, nil
	case assistant.ContextTask:
		task, err := a.Work.GetTask(ctx, workScope, contextRef.ID)
		if err != nil {
			return assistant.Evidence{}, err
		}
		due := "No due date recorded."
		if task.DueAt != nil {
			due = task.DueAt.UTC().Format(time.RFC3339)
		}
		return assistant.Evidence{
			Scope:    assistant.Scope{WorkspaceID: scope.ID, UserID: workScope.UserID},
			ID:       "work-task-" + task.ID,
			SourceID: "task:" + task.ID,
			Title:    task.Title,
			URI:      "work://task/" + task.ID,
			Snippet: boundEvidenceText(strings.Join([]string{
				"Task: " + task.Title,
				"Status: " + string(task.Status),
				"Priority: " + string(task.Priority),
				"Due: " + due,
				"Description: " + firstNonEmpty(task.Description, "No description recorded."),
			}, "\n")),
			Freshness: assistant.FreshCurrent,
		}, nil
	case assistant.ContextDecision:
		decision, err := a.Work.GetDecision(ctx, workScope, contextRef.ID)
		if err != nil {
			return assistant.Evidence{}, err
		}
		return assistant.Evidence{
			Scope:    assistant.Scope{WorkspaceID: scope.ID, UserID: workScope.UserID},
			ID:       "work-decision-" + decision.ID,
			SourceID: "decision:" + decision.ID,
			Title:    decision.Title,
			URI:      "work://decision/" + decision.ID,
			Snippet: boundEvidenceText(strings.Join([]string{
				"Decision: " + decision.Title,
				"Status: " + string(decision.Status),
				"Context: " + firstNonEmpty(decision.Context, "No context recorded."),
				"Outcome: " + firstNonEmpty(decision.Outcome, "No outcome recorded."),
				"Rationale: " + firstNonEmpty(decision.Rationale, "No rationale recorded."),
			}, "\n")),
			Freshness: assistant.FreshCurrent,
		}, nil
	case assistant.ContextSource:
		detail, err := a.Knowledge.GetSourceDetail(ctx, scope, contextRef.ID)
		if err != nil {
			return assistant.Evidence{}, err
		}
		lines := []string{"Source: " + detail.Source.Name, "Kind: " + detail.Source.Kind}
		if detail.Source.URI != "" {
			lines = append(lines, "URI: "+detail.Source.URI)
		}
		for index, chunk := range detail.Chunks {
			if index >= 8 {
				break
			}
			lines = append(lines, "Excerpt: "+chunk.Text)
		}
		revisionID := ""
		if len(detail.Revisions) > 0 {
			revisionID = detail.Revisions[0].ID
		}
		return assistant.Evidence{
			Scope:      assistant.Scope{WorkspaceID: scope.ID, UserID: workScope.UserID},
			ID:         "knowledge-source-" + detail.Source.ID,
			SourceID:   detail.Source.ID,
			RevisionID: revisionID,
			Title:      detail.Source.Name,
			URI:        detail.Source.URI,
			Snippet:    boundEvidenceText(strings.Join(lines, "\n")),
			Freshness:  assistant.FreshCurrent,
		}, nil
	default:
		return assistant.Evidence{}, fmt.Errorf("%w: unsupported assistant context", ErrInvalidInput)
	}
}

func searchHitFromEvidence(evidence assistant.Evidence) SearchHit {
	freshness := string(assistant.FreshUnknown)
	if evidence.Freshness == assistant.FreshCurrent {
		freshness = "current"
	} else if evidence.Freshness == assistant.FreshStale {
		freshness = "stale"
	}
	return SearchHit{ID: evidence.ID, SourceID: evidence.SourceID, SourceName: evidence.Title, RevisionID: evidence.RevisionID, URI: evidence.URI, Excerpt: evidence.Snippet, Freshness: freshness, Origin: "canonical", RetrievalMode: "context"}
}

func boundEvidenceText(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 8_000 {
		return string(runes[:8_000])
	}
	return value
}

const knowledgeStaleAfter = 30 * 24 * time.Hour

func revisionFreshness(revision knowledge.SourceRevision, now time.Time) string {
	modified := revision.ModifiedAt
	if modified.IsZero() {
		modified = revision.IngestedAt
	}
	if modified.IsZero() || now.IsZero() || now.Sub(modified) < knowledgeStaleAfter {
		return "current"
	}
	return "stale"
}

func (a *App) now() time.Time {
	return time.Now().UTC()
}

type deterministicGenerator struct{}

func (deterministicGenerator) Generate(_ context.Context, request assistant.GenerationRequest) (assistant.Draft, error) {
	if len(request.Evidence) == 0 {
		return assistant.Draft{}, errors.New("no evidence")
	}
	evidence := request.Evidence[0]
	return assistant.Draft{Answer: "Verified context: " + evidence.Snippet, Citations: []assistant.Citation{{EvidenceID: evidence.ID, Quote: evidence.Snippet, Locator: evidence.URI}}}, nil
}

func sourceSummary(source knowledge.KnowledgeSource) SourceSummary {
	return SourceSummary{ID: source.ID, Title: source.Name, Kind: source.Kind, URI: source.URI, UpdatedAt: source.UpdatedAt.UTC()}
}

func importedSourceFromProjection(projection knowledge.ManualImportProjection) ImportedSource {
	return ImportedSource{
		Source: SourceSummary{
			ID: projection.Source.ID, Title: projection.Source.Title,
			Kind: projection.Source.Kind, URI: projection.Source.URI,
			UpdatedAt: projection.Source.UpdatedAt.UTC(),
		},
		RevisionID: projection.RevisionID,
		ChunkID:    projection.ChunkID,
	}
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	if len(value) > 120 {
		return value[:120]
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func searchTerms(query string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, term := range strings.Fields(strings.ToLower(strings.TrimSpace(query))) {
		if len(term) < 2 {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		result = append(result, term)
	}
	return result
}

func matchesTerms(terms []string, value string) bool {
	if len(terms) == 0 {
		return false
	}
	lower := strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

var (
	ErrInvalidInput                = errors.New("application input is invalid")
	ErrConversationNotFound        = errors.New("assistant conversation not found")
	ErrConversationContextConflict = errors.New("assistant conversation context cannot be changed")
)
