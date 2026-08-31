package knowledge

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMemoryManualImportIdempotencyBoundaries(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	scope := WorkspaceScope{ID: "idempotency-workspace"}

	if _, found, err := repository.LookupManualImportIdempotency(ctx, WorkspaceScope{}, "user-1", "key-1"); !errors.Is(err, ErrInvalidInput) || found {
		t.Fatalf("invalid lookup scope = found:%v error:%v", found, err)
	}
	if _, found, err := repository.LookupManualImportIdempotency(ctx, scope, " ", "key-1"); !errors.Is(err, ErrInvalidInput) || found {
		t.Fatalf("invalid lookup user = found:%v error:%v", found, err)
	}
	if _, found, err := repository.LookupManualImportIdempotency(ctx, scope, "user-1", " "); !errors.Is(err, ErrInvalidInput) || found {
		t.Fatalf("invalid lookup key = found:%v error:%v", found, err)
	}
	if _, found, err := repository.LookupManualImportIdempotency(ctx, scope, "user-1", "key-1"); err != nil || found {
		t.Fatalf("missing idempotency record = found:%v error:%v", found, err)
	}

	record := ManualImportIdempotencyRecord{
		WorkspaceID: scope.ID, UserID: "user-1", Operation: "knowledge.import_manual",
		IdempotencyKey: "key-1", RequestHash: "hash-1", ResponseJSON: []byte(`{"sourceId":"source-1"}`),
	}
	if err := repository.SaveManualImportIdempotency(ctx, WorkspaceScope{}, record); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid save scope = %v", err)
	}
	mismatched := record
	mismatched.WorkspaceID = "other-workspace"
	if err := repository.SaveManualImportIdempotency(ctx, scope, mismatched); !errors.Is(err, ErrWorkspaceMismatch) {
		t.Fatalf("mismatched save scope = %v", err)
	}
	for _, invalid := range []ManualImportIdempotencyRecord{
		{WorkspaceID: scope.ID, UserID: "", Operation: record.Operation, IdempotencyKey: record.IdempotencyKey, RequestHash: record.RequestHash, ResponseJSON: record.ResponseJSON},
		{WorkspaceID: scope.ID, UserID: record.UserID, Operation: "", IdempotencyKey: record.IdempotencyKey, RequestHash: record.RequestHash, ResponseJSON: record.ResponseJSON},
		{WorkspaceID: scope.ID, UserID: record.UserID, Operation: record.Operation, IdempotencyKey: "", RequestHash: record.RequestHash, ResponseJSON: record.ResponseJSON},
		{WorkspaceID: scope.ID, UserID: record.UserID, Operation: record.Operation, IdempotencyKey: record.IdempotencyKey, RequestHash: "", ResponseJSON: record.ResponseJSON},
	} {
		if err := repository.SaveManualImportIdempotency(ctx, scope, invalid); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid idempotency record = %v", err)
		}
	}
	invalidOperation := record
	invalidOperation.Operation = "other.operation"
	if err := repository.SaveManualImportIdempotency(ctx, scope, invalidOperation); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid operation = %v", err)
	}
	noResponse := record
	noResponse.ResponseJSON = nil
	if err := repository.SaveManualImportIdempotency(ctx, scope, noResponse); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing response = %v", err)
	}

	if err := repository.SaveManualImportIdempotency(ctx, scope, record); err != nil {
		t.Fatalf("save idempotency record = %v", err)
	}
	if err := repository.SaveManualImportIdempotency(ctx, scope, record); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate idempotency record = %v", err)
	}
	got, found, err := repository.LookupManualImportIdempotency(ctx, scope, record.UserID, record.IdempotencyKey)
	if err != nil || !found || string(got.ResponseJSON) != string(record.ResponseJSON) {
		t.Fatalf("stored idempotency record = %#v, found:%v error:%v", got, found, err)
	}
	got.ResponseJSON[0] = 'X'
	again, found, err := repository.LookupManualImportIdempotency(ctx, scope, record.UserID, record.IdempotencyKey)
	if err != nil || !found || string(again.ResponseJSON) != string(record.ResponseJSON) {
		t.Fatalf("idempotency response was not cloned = %#v, found:%v error:%v", again, found, err)
	}
}

func TestMemoryRepositoryListAndSearchOrderingBranches(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	scope := WorkspaceScope{ID: "ordering-workspace"}
	now := time.Date(2026, 8, 30, 8, 0, 0, 0, time.UTC)

	if _, err := repository.ListSourceItems(ctx, WorkspaceScope{}, "source", 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid ListSourceItems scope = %v", err)
	}
	if _, err := repository.ListRevisions(ctx, WorkspaceScope{}, "item", 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid ListRevisions scope = %v", err)
	}
	if _, err := repository.ListChunks(ctx, WorkspaceScope{}, "revision", 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid ListChunks scope = %v", err)
	}
	if _, err := repository.ListEvidenceForSource(ctx, WorkspaceScope{}, "source", 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid ListEvidenceForSource scope = %v", err)
	}

	sourceOld := boundaryKnowledgeSource("source-old", scope.ID, now)
	sourceNew := boundaryKnowledgeSource("source-new", scope.ID, now.Add(time.Hour))
	for _, source := range []KnowledgeSource{sourceOld, sourceNew} {
		if err := repository.CreateSource(ctx, scope, source); err != nil {
			t.Fatalf("CreateSource(%s) = %v", source.ID, err)
		}
	}
	if got, err := repository.ListSources(ctx, scope, 1); err != nil || len(got) != 1 || got[0].ID != sourceNew.ID {
		t.Fatalf("ordered sources = %#v, error:%v", got, err)
	}

	itemOld := boundaryKnowledgeItem("item-old", scope.ID, sourceNew.ID, now)
	itemNew := boundaryKnowledgeItem("item-new", scope.ID, sourceNew.ID, now.Add(time.Hour))
	for _, item := range []SourceItem{itemOld, itemNew} {
		if err := repository.CreateSourceItem(ctx, scope, item); err != nil {
			t.Fatalf("CreateSourceItem(%s) = %v", item.ID, err)
		}
	}
	if got, err := repository.ListSourceItems(ctx, scope, sourceNew.ID, 1); err != nil || len(got) != 1 || got[0].ID != itemNew.ID {
		t.Fatalf("ordered source items = %#v, error:%v", got, err)
	}

	revisionOld := boundaryKnowledgeRevision("revision-old", scope.ID, itemNew.ID, "old text", now)
	revisionNew := boundaryKnowledgeRevision("revision-new", scope.ID, itemNew.ID, "new text", now.Add(time.Hour))
	for _, revision := range []SourceRevision{revisionOld, revisionNew} {
		if err := repository.CreateRevision(ctx, scope, revision); err != nil {
			t.Fatalf("CreateRevision(%s) = %v", revision.ID, err)
		}
	}
	if got, err := repository.ListRevisions(ctx, scope, itemNew.ID, 1); err != nil || len(got) != 1 || got[0].ID != revisionNew.ID {
		t.Fatalf("ordered revisions = %#v, error:%v", got, err)
	}

	for _, chunk := range []KnowledgeChunk{
		boundaryKnowledgeChunk("chunk-b", scope.ID, revisionNew.ID, 0, "needle", now),
		boundaryKnowledgeChunk("chunk-a", scope.ID, revisionNew.ID, 0, "needle", now),
		boundaryKnowledgeChunk("chunk-c", scope.ID, revisionNew.ID, 1, "needle", now),
	} {
		if err := repository.CreateChunk(ctx, scope, chunk); err != nil {
			t.Fatalf("CreateChunk(%s) = %v", chunk.ID, err)
		}
	}
	if got, err := repository.ListChunks(ctx, scope, revisionNew.ID, 1); err != nil || len(got) != 1 || got[0].ID != "chunk-a" {
		t.Fatalf("ordered chunks = %#v, error:%v", got, err)
	}
	if err := repository.SetCurrentRevision(ctx, scope, itemNew.ID, revisionNew.ID); err != nil {
		t.Fatalf("SetCurrentRevision() = %v", err)
	}
	repository.mu.Lock()
	itemWithoutCurrentRevision := repository.items[scopedID{scope.ID, itemNew.ID}]
	itemWithoutCurrentRevision.CurrentRevisionID = ""
	repository.items[scopedID{scope.ID, itemNew.ID}] = itemWithoutCurrentRevision
	repository.mu.Unlock()

	if _, err := repository.ListEvidenceForSource(ctx, scope, "missing-source", 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing evidence source = %v", err)
	}
	repository.mu.Lock()
	repository.evidence[scopedID{scope.ID, "claim-ordering"}] = []ClaimEvidence{
		{ID: "evidence-c", WorkspaceID: scope.ID, SourceRevisionID: revisionNew.ID, CreatedAt: now.Add(time.Hour)},
		{ID: "evidence-b", WorkspaceID: scope.ID, SourceRevisionID: revisionNew.ID, CreatedAt: now},
		{ID: "evidence-a", WorkspaceID: scope.ID, SourceRevisionID: revisionNew.ID, CreatedAt: now},
	}
	repository.mu.Unlock()
	if got, err := repository.ListEvidenceForSource(ctx, scope, sourceNew.ID, 1); err != nil || len(got) != 1 || got[0].ID != "evidence-a" {
		t.Fatalf("ordered source evidence = %#v, error:%v", got, err)
	}

	searchItem := boundaryKnowledgeItem("search-item", scope.ID, sourceNew.ID, now)
	searchItem.CurrentRevisionID = revisionNew.ID
	if err := repository.CreateSourceItem(ctx, scope, searchItem); err != nil {
		t.Fatalf("CreateSource(search item) = %v", err)
	}
	searchRevision := boundaryKnowledgeRevision("search-revision", scope.ID, searchItem.ID, "search content", now)
	if err := repository.CreateRevision(ctx, scope, searchRevision); err != nil {
		t.Fatalf("CreateRevision(search) = %v", err)
	}
	searchItem.CurrentRevisionID = searchRevision.ID
	repository.mu.Lock()
	repository.items[scopedID{scope.ID, searchItem.ID}] = searchItem
	repository.items[scopedID{scope.ID, "orphan-search-item"}] = SourceItem{
		ID: "orphan-search-item", WorkspaceID: scope.ID, SourceID: sourceNew.ID, CurrentRevisionID: "missing-search-revision",
	}
	repository.chunks[scopedID{scope.ID, "search-hit-b"}] = KnowledgeChunk{ID: "search-hit-b", WorkspaceID: scope.ID, RevisionID: searchRevision.ID, Ordinal: 0, Text: "needle"}
	repository.chunks[scopedID{scope.ID, "search-hit-a"}] = KnowledgeChunk{ID: "search-hit-a", WorkspaceID: scope.ID, RevisionID: searchRevision.ID, Ordinal: 0, Text: "needle"}
	repository.chunks[scopedID{scope.ID, "search-miss"}] = KnowledgeChunk{ID: "search-miss", WorkspaceID: scope.ID, RevisionID: searchRevision.ID, Ordinal: 1, Text: "unrelated"}
	repository.mu.Unlock()
	results, err := repository.Search(ctx, scope, "needle", 10)
	if err != nil || len(results) != 2 || results[0].Chunk.ID != "search-hit-a" || results[1].Chunk.ID != "search-hit-b" {
		t.Fatalf("ordered search results = %#v, error:%v", results, err)
	}
	if _, err := repository.Search(ctx, scope, "needle", 1); err != nil {
		t.Fatalf("bounded search = %v", err)
	}
}

func TestKnowledgeEmbeddingProviderBoundaryBranches(t *testing.T) {
	var nilProvider *HTTPEmbeddingProvider
	if _, err := nilProvider.Embed(context.Background(), "text"); !errors.Is(err, ErrEmbeddingUnavailable) {
		t.Fatalf("nil provider error = %v", err)
	}
	if _, err := (&HTTPEmbeddingProvider{BaseURL: "  "}).Embed(context.Background(), "text"); !errors.Is(err, ErrEmbeddingUnavailable) {
		t.Fatalf("blank provider URL error = %v", err)
	}
	if _, err := (&HTTPEmbeddingProvider{BaseURL: "http://embedding.local"}).Embed(context.Background(), " "); err == nil {
		t.Fatal("blank embedding text unexpectedly succeeded")
	}
	if _, err := (&HTTPEmbeddingProvider{BaseURL: "http://[::1"}).Embed(context.Background(), "text"); err == nil {
		t.Fatal("invalid embedding URL unexpectedly succeeded")
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"embedding":[` + strings.Trim(strings.Repeat("0,", EmbeddingDimensions), ",") + `]}`))
	}))
	serverURL := server.URL
	server.Close()
	if _, err := (&HTTPEmbeddingProvider{BaseURL: serverURL}).Embed(context.Background(), "text"); !errors.Is(err, ErrEmbeddingUnavailable) {
		t.Fatalf("closed embedding server error = %v", err)
	}

	workingServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"embedding":[` + strings.Trim(strings.Repeat("0,", EmbeddingDimensions), ",") + `]}`))
	}))
	defer workingServer.Close()
	provider := &HTTPEmbeddingProvider{BaseURL: workingServer.URL}
	if embedding, err := provider.Embed(context.Background(), "text"); err != nil || len(embedding) != EmbeddingDimensions {
		t.Fatalf("default HTTP client embedding = %d, error:%v", len(embedding), err)
	}
}

func TestKnowledgeServiceSourceReadFailureBranches(t *testing.T) {
	ctx := context.Background()
	scope := WorkspaceScope{ID: "service-coverage-workspace"}
	if _, err := (&Service{repository: &fakeRepository{}}).GetSource(ctx, scope, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid GetSource identifier = %v", err)
	}

	for _, testCase := range []struct {
		name  string
		setup func(*serviceCapabilityRepository)
		want  string
	}{
		{name: "source", setup: func(repository *serviceCapabilityRepository) { repository.getSourceErr = errors.New("source failed") }, want: "source"},
		{name: "items", setup: func(repository *serviceCapabilityRepository) {
			repository.listSourceItemsErr = errors.New("items failed")
		}, want: "items"},
		{name: "revisions", setup: func(repository *serviceCapabilityRepository) {
			repository.sourceItems = []SourceItem{{ID: "item-1"}}
			repository.listRevisionsErr = errors.New("revisions failed")
		}, want: "revisions"},
		{name: "chunks", setup: func(repository *serviceCapabilityRepository) {
			repository.sourceItems = []SourceItem{{ID: "item-1"}}
			repository.revisions = []SourceRevision{{ID: "revision-1"}}
			repository.listChunksErr = errors.New("chunks failed")
		}, want: "chunks"},
		{name: "evidence", setup: func(repository *serviceCapabilityRepository) {
			repository.listEvidenceErr = errors.New("evidence failed")
		}, want: "evidence"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repository := newServiceBoundaryRepository()
			testCase.setup(repository)
			service, err := NewService(repository)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.GetSourceDetail(ctx, scope, "source-1"); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("GetSourceDetail() error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func (repository *serviceCapabilityRepository) ListEvidenceForSource(_ context.Context, _ WorkspaceScope, _ string, _ int) ([]ClaimEvidence, error) {
	return repository.evidence, repository.listEvidenceErr
}

func TestKnowledgeChunkValidationRejectsNonFiniteFixedDimensionEmbedding(t *testing.T) {
	chunk := KnowledgeChunk{ID: "chunk", WorkspaceID: "workspace", RevisionID: "revision", Text: "text", Embedding: make([]float32, EmbeddingDimensions)}
	chunk.Embedding[EmbeddingDimensions-1] = float32(math.NaN())
	if err := chunk.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("non-finite fixed-dimension embedding error = %v", err)
	}
}
