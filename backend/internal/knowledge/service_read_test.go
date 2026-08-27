package knowledge

import (
	"context"
	"errors"
	"testing"
)

type readTrackingRepository struct {
	fakeRepository
	chunkCalls int
	claimCalls int
	chunkScope WorkspaceScope
	claimScope WorkspaceScope
	chunkID    string
	claimID    string
}

func (repository *readTrackingRepository) GetChunk(_ context.Context, scope WorkspaceScope, id string) (KnowledgeChunk, error) {
	repository.chunkCalls++
	repository.chunkScope = scope
	repository.chunkID = id
	return KnowledgeChunk{ID: id, WorkspaceID: scope.ID, RevisionID: "revision-1", Text: "chunk"}, nil
}

func (repository *readTrackingRepository) GetClaim(_ context.Context, scope WorkspaceScope, id string) (KnowledgeClaim, error) {
	repository.claimCalls++
	repository.claimScope = scope
	repository.claimID = id
	return KnowledgeClaim{ID: id, WorkspaceID: scope.ID, Statement: "claim", Certainty: ClaimInferred, Freshness: ClaimFreshnessUnknown}, nil
}

func TestServiceReadMethodsValidateAndDelegateWorkspaceScope(t *testing.T) {
	repository := &readTrackingRepository{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewWorkspaceScope("workspace-a")
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := service.GetChunk(context.Background(), scope, "chunk-1")
	if err != nil || chunk.ID != "chunk-1" || repository.chunkScope != scope || repository.chunkID != "chunk-1" {
		t.Fatalf("GetChunk() = %#v, %v, repository=%#v", chunk, err, repository)
	}
	claim, err := service.GetClaim(context.Background(), scope, "claim-1")
	if err != nil || claim.ID != "claim-1" || repository.claimScope != scope || repository.claimID != "claim-1" {
		t.Fatalf("GetClaim() = %#v, %v, repository=%#v", claim, err, repository)
	}
	if repository.chunkCalls != 1 || repository.claimCalls != 1 {
		t.Fatalf("read calls = chunk:%d claim:%d", repository.chunkCalls, repository.claimCalls)
	}
}

func TestServiceReadMethodsRejectInvalidScopeOrIDBeforeRepository(t *testing.T) {
	repository := &readTrackingRepository{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		call func() error
	}{
		{name: "chunk blank id", call: func() error {
			_, err := service.GetChunk(context.Background(), WorkspaceScope{ID: "workspace-a"}, " ")
			return err
		}},
		{name: "claim blank id", call: func() error {
			_, err := service.GetClaim(context.Background(), WorkspaceScope{ID: "workspace-a"}, " ")
			return err
		}},
		{name: "chunk blank scope", call: func() error {
			_, err := service.GetChunk(context.Background(), WorkspaceScope{}, "chunk-1")
			return err
		}},
		{name: "claim blank scope", call: func() error {
			_, err := service.GetClaim(context.Background(), WorkspaceScope{}, "claim-1")
			return err
		}},
	}
	for _, testCase := range cases {
		if err := testCase.call(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s error = %v, want ErrInvalidInput", testCase.name, err)
		}
	}
	if repository.chunkCalls != 0 || repository.claimCalls != 0 {
		t.Fatalf("repository was called for invalid reads: chunk:%d claim:%d", repository.chunkCalls, repository.claimCalls)
	}
}
