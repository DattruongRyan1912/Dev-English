package knowledge

import (
	"context"
	"errors"
	"testing"
)

type serviceCapabilityRepository struct {
	fakeRepository

	getSourceErr       error
	listSourcesErr     error
	getSourceItemErr   error
	listSourceItemsErr error
	getRevisionErr     error
	listRevisionsErr   error
	getChunkErr        error
	listChunksErr      error
	searchErr          error
	listEvidenceErr    error
	getClaimErr        error
	listSources        []KnowledgeSource
	source             KnowledgeSource
	sourceItem         SourceItem
	sourceItems        []SourceItem
	revision           SourceRevision
	revisions          []SourceRevision
	chunk              KnowledgeChunk
	chunks             []KnowledgeChunk
	claim              KnowledgeClaim
	evidence           []ClaimEvidence
	searchResults      []SearchResult
	searchCalls        int
}

func (repository *serviceCapabilityRepository) GetSource(_ context.Context, _ WorkspaceScope, _ string) (KnowledgeSource, error) {
	return repository.source, repository.getSourceErr
}

func (repository *serviceCapabilityRepository) ListSources(_ context.Context, _ WorkspaceScope, _ int) ([]KnowledgeSource, error) {
	return repository.listSources, repository.listSourcesErr
}

func (repository *serviceCapabilityRepository) GetSourceItem(_ context.Context, _ WorkspaceScope, _ string) (SourceItem, error) {
	return repository.sourceItem, repository.getSourceItemErr
}

func (repository *serviceCapabilityRepository) ListSourceItems(_ context.Context, _ WorkspaceScope, _ string, _ int) ([]SourceItem, error) {
	return repository.sourceItems, repository.listSourceItemsErr
}

func (repository *serviceCapabilityRepository) GetRevision(_ context.Context, _ WorkspaceScope, _ string) (SourceRevision, error) {
	return repository.revision, repository.getRevisionErr
}

func (repository *serviceCapabilityRepository) ListRevisions(_ context.Context, _ WorkspaceScope, _ string, _ int) ([]SourceRevision, error) {
	return repository.revisions, repository.listRevisionsErr
}

func (repository *serviceCapabilityRepository) GetChunk(_ context.Context, _ WorkspaceScope, _ string) (KnowledgeChunk, error) {
	return repository.chunk, repository.getChunkErr
}

func (repository *serviceCapabilityRepository) ListChunks(_ context.Context, _ WorkspaceScope, _ string, _ int) ([]KnowledgeChunk, error) {
	return repository.chunks, repository.listChunksErr
}

func (repository *serviceCapabilityRepository) Search(_ context.Context, _ WorkspaceScope, _ string, _ int) ([]SearchResult, error) {
	repository.searchCalls++
	return repository.searchResults, repository.searchErr
}

func (repository *serviceCapabilityRepository) GetClaim(_ context.Context, _ WorkspaceScope, _ string) (KnowledgeClaim, error) {
	return repository.claim, repository.getClaimErr
}

func (repository *serviceCapabilityRepository) ListClaimEvidence(_ context.Context, _ WorkspaceScope, _ string) ([]ClaimEvidence, error) {
	return repository.evidence, repository.listEvidenceErr
}

func newServiceBoundaryRepository() *serviceCapabilityRepository {
	return &serviceCapabilityRepository{
		source:     KnowledgeSource{ID: "source-1", WorkspaceID: "workspace-a"},
		sourceItem: SourceItem{ID: "item-1", WorkspaceID: "workspace-a"},
		revision:   SourceRevision{ID: "revision-1", WorkspaceID: "workspace-a"},
		chunk:      KnowledgeChunk{ID: "chunk-1", WorkspaceID: "workspace-a"},
		claim:      KnowledgeClaim{ID: "claim-1", WorkspaceID: "workspace-a"},
	}
}

func TestServiceReadCapabilitiesFailClosedAndPropagateRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	scope := WorkspaceScope{ID: "workspace-a"}

	service, err := NewService(&fakeRepository{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListSources(ctx, scope, 10); !errors.Is(err, ErrUnsupportedRead) {
		t.Fatalf("unsupported ListSources() error = %v, want ErrUnsupportedRead", err)
	}
	if _, err := service.GetSourceDetail(ctx, scope, "source-1"); !errors.Is(err, ErrUnsupportedRead) {
		t.Fatalf("unsupported GetSourceDetail() error = %v, want ErrUnsupportedRead", err)
	}
	if _, err := service.Search(ctx, scope, "query", 10); !errors.Is(err, ErrUnsupportedRead) {
		t.Fatalf("unsupported Search() error = %v, want ErrUnsupportedRead", err)
	}
	if _, err := NewService(nil); !errors.Is(err, ErrNilRepository) {
		t.Fatalf("nil service repository error = %v, want ErrNilRepository", err)
	}

	readCases := []struct {
		name string
		call func(*Service) error
		make func() *serviceCapabilityRepository
	}{
		{name: "list sources", call: func(service *Service) error {
			_, err := service.ListSources(ctx, scope, 10)
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.listSourcesErr = errors.New("list sources failed")
			return repository
		}},
		{name: "get source", call: func(service *Service) error {
			_, err := service.GetSource(ctx, scope, "source-1")
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.getSourceErr = errors.New("get source failed")
			return repository
		}},
		{name: "get source item", call: func(service *Service) error {
			_, err := service.GetSourceItem(ctx, scope, "item-1")
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.getSourceItemErr = errors.New("get source item failed")
			return repository
		}},
		{name: "get revision", call: func(service *Service) error {
			_, err := service.GetRevision(ctx, scope, "revision-1")
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.getRevisionErr = errors.New("get revision failed")
			return repository
		}},
		{name: "get chunk", call: func(service *Service) error {
			_, err := service.GetChunk(ctx, scope, "chunk-1")
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.getChunkErr = errors.New("get chunk failed")
			return repository
		}},
		{name: "get claim", call: func(service *Service) error {
			_, err := service.GetClaim(ctx, scope, "claim-1")
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.getClaimErr = errors.New("get claim failed")
			return repository
		}},
		{name: "list claim evidence", call: func(service *Service) error {
			_, err := service.ListClaimEvidence(ctx, scope, "claim-1")
			return err
		}, make: func() *serviceCapabilityRepository {
			repository := newServiceBoundaryRepository()
			repository.listEvidenceErr = errors.New("list evidence failed")
			return repository
		}},
	}
	for _, testCase := range readCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := testCase.make()
			service, err := NewService(repository)
			if err != nil {
				t.Fatal(err)
			}
			if err := testCase.call(service); err == nil {
				t.Fatalf("%s unexpectedly succeeded", testCase.name)
			}
		})
	}

	detailCases := []struct {
		name string
		set  func(*serviceCapabilityRepository)
	}{
		{name: "source lookup", set: func(repository *serviceCapabilityRepository) {
			repository.getSourceErr = errors.New("source lookup failed")
		}},
		{name: "item listing", set: func(repository *serviceCapabilityRepository) {
			repository.listSourceItemsErr = errors.New("item listing failed")
		}},
		{name: "revision listing", set: func(repository *serviceCapabilityRepository) {
			repository.sourceItems = []SourceItem{{ID: "item-1"}}
			repository.listRevisionsErr = errors.New("revision listing failed")
		}},
		{name: "chunk listing", set: func(repository *serviceCapabilityRepository) {
			repository.sourceItems = []SourceItem{{ID: "item-1"}}
			repository.revisions = []SourceRevision{{ID: "revision-1"}}
			repository.listChunksErr = errors.New("chunk listing failed")
		}},
		{name: "evidence listing", set: func(repository *serviceCapabilityRepository) {
			repository.listEvidenceErr = errors.New("evidence listing failed")
		}},
	}
	for _, testCase := range detailCases {
		t.Run("detail "+testCase.name, func(t *testing.T) {
			repository := newServiceBoundaryRepository()
			testCase.set(repository)
			service, err := NewService(repository)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.GetSourceDetail(ctx, scope, "source-1"); err == nil {
				t.Fatalf("detail %s unexpectedly succeeded", testCase.name)
			}
		})
	}

	repository := newServiceBoundaryRepository()
	repository.searchErr = errors.New("search failed")
	service, err = NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Search(ctx, scope, "query", 10); !errors.Is(err, repository.searchErr) {
		t.Fatalf("Search() error = %v, want repository error", err)
	}
	if _, err := service.Search(ctx, scope, "  ", 10); err != nil {
		t.Fatalf("blank Search() error = %v, want empty result", err)
	}
	if repository.searchCalls != 1 {
		t.Fatalf("blank Search() called repository; calls = %d, want 1", repository.searchCalls)
	}
}

func TestServiceEntryPointsRejectInvalidScopeAndIdentifiers(t *testing.T) {
	ctx := context.Background()
	validScope := WorkspaceScope{ID: "workspace-a"}
	invalidScope := WorkspaceScope{}
	repository := &fakeRepository{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{name: "create source scope", call: func() error { return service.CreateSource(ctx, invalidScope, KnowledgeSource{}) }},
		{name: "list sources scope", call: func() error { _, err := service.ListSources(ctx, invalidScope, 10); return err }},
		{name: "source detail id", call: func() error { _, err := service.GetSourceDetail(ctx, validScope, " "); return err }},
		{name: "search scope", call: func() error { _, err := service.Search(ctx, invalidScope, "query", 10); return err }},
		{name: "create item scope", call: func() error { return service.CreateSourceItem(ctx, invalidScope, SourceItem{}) }},
		{name: "get item id", call: func() error { _, err := service.GetSourceItem(ctx, validScope, " "); return err }},
		{name: "append revision scope", call: func() error { return service.AppendRevision(ctx, invalidScope, SourceRevision{}) }},
		{name: "get revision id", call: func() error { _, err := service.GetRevision(ctx, validScope, " "); return err }},
		{name: "create chunk scope", call: func() error { return service.CreateChunk(ctx, invalidScope, KnowledgeChunk{}) }},
		{name: "get chunk id", call: func() error { _, err := service.GetChunk(ctx, validScope, " "); return err }},
		{name: "create topic scope", call: func() error { return service.CreateTopic(ctx, invalidScope, Topic{}) }},
		{name: "create claim scope", call: func() error { return service.CreateClaimBundle(ctx, invalidScope, KnowledgeClaim{}, nil) }},
		{name: "get claim id", call: func() error { _, err := service.GetClaim(ctx, validScope, " "); return err }},
		{name: "list evidence id", call: func() error { _, err := service.ListClaimEvidence(ctx, validScope, " "); return err }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}
