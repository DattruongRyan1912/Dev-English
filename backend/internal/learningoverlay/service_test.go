package learningoverlay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestServiceRecordsAndListsOnlyWithinScope(t *testing.T) {
	repository := NewMemoryRepository()
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{WorkspaceID: "workspace-a", UserID: "user-a"}
	first, err := service.Record(context.Background(), scope, ObservationInput{
		SourceType: "task",
		SourceID:   "task-1",
		Skill:      "technical_writing",
		Prompt:     "Explain the next step.",
		Response:   "I will inspect the backend logs.",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Record(context.Background(), scope, ObservationInput{
		SourceType: "decision",
		SourceID:   "decision-1",
		Skill:      "speaking",
		Prompt:     "State the trade-off.",
		Response:   "We choose the simpler path.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("observation ids must be unique")
	}
	items, err := service.List(context.Background(), scope, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID {
		t.Fatalf("items = %#v, want newest first", items)
	}
	other, err := service.List(context.Background(), Scope{WorkspaceID: "workspace-b", UserID: "user-b"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("cross-scope observations leaked: %#v", other)
	}
}

func TestServiceRejectsInvalidAndCanceledRequests(t *testing.T) {
	service, err := NewService(NewMemoryRepository())
	if err != nil {
		t.Fatal(err)
	}
	valid := ObservationInput{Skill: "speaking", Prompt: "Prompt", Response: "Response"}
	if _, err := service.Record(context.Background(), Scope{WorkspaceID: "", UserID: "user"}, valid); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid scope error = %v", err)
	}
	if _, err := service.Record(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, ObservationInput{Skill: "speaking", Prompt: "Prompt"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid observation error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Record(ctx, Scope{WorkspaceID: "workspace", UserID: "user"}, valid); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled record error = %v", err)
	}
}

func TestObservationInputLimitsFields(t *testing.T) {
	tooLong := make([]byte, 20001)
	for index := range tooLong {
		tooLong[index] = 'x'
	}
	if err := (ObservationInput{Skill: "speaking", Prompt: string(tooLong), Response: "ok"}).validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("long input error = %v", err)
	}
	if err := (ObservationInput{SourceType: "bad\nsource", Skill: "speaking", Prompt: "ok", Response: "ok"}).validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid source error = %v", err)
	}
	_ = time.Now()
}

type learningRepositoryStub struct {
	saveErr error
	listErr error
	items   []Observation
	limit   int
}

func (r *learningRepositoryStub) Save(context.Context, Scope, Observation) error { return r.saveErr }

func (r *learningRepositoryStub) List(_ context.Context, _ Scope, limit int) ([]Observation, error) {
	r.limit = limit
	return append([]Observation(nil), r.items...), r.listErr
}

func TestLearningServiceCoversRepositoryErrorsAndNormalization(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("nil repository unexpectedly accepted")
	}
	saveErr := errors.New("save failed")
	stub := &learningRepositoryStub{saveErr: saveErr}
	service, err := NewService(stub)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 8, 28, 10, 0, 0, 0, time.FixedZone("local", 7*60*60)) }
	scope := Scope{WorkspaceID: " workspace ", UserID: " user "}
	input := ObservationInput{SourceType: " task ", SourceID: " task-1 ", Skill: " speaking ", Prompt: " prompt ", Response: " response ", Feedback: " feedback "}
	if _, err := service.Record(context.Background(), scope, input); !errors.Is(err, saveErr) {
		t.Fatalf("repository save error = %v", err)
	}
	if _, err := service.Record(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, ObservationInput{Skill: "speaking", Prompt: "prompt", Response: "response"}); !errors.Is(err, saveErr) {
		t.Fatalf("second repository save error = %v", err)
	}

	stub.saveErr = nil
	stub.items = []Observation{{ID: "one"}}
	stub.listErr = errors.New("list failed")
	if _, err := service.List(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, 0); !errors.Is(err, stub.listErr) {
		t.Fatalf("repository list error = %v", err)
	}
	if stub.limit != 50 {
		t.Fatalf("default list limit = %d, want 50", stub.limit)
	}
	if _, err := service.List(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, 201); !errors.Is(err, stub.listErr) {
		t.Fatalf("large-limit list error = %v", err)
	}
	if stub.limit != 50 {
		t.Fatalf("large-limit normalized to %d, want 50", stub.limit)
	}
}

func TestLearningMemoryRepositoryRejectsInvalidContextScopeAndDuplicates(t *testing.T) {
	repository := NewMemoryRepository()
	observation := Observation{ID: "observation-1", WorkspaceID: "workspace", UserID: "user", CreatedAt: time.Now().UTC()}
	if err := repository.Save(nil, Scope{WorkspaceID: "workspace", UserID: "user"}, observation); err == nil {
		t.Fatal("nil context unexpectedly accepted")
	}
	if err := repository.Save(context.Background(), Scope{WorkspaceID: "other", UserID: "user"}, observation); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("scope mismatch error = %v", err)
	}
	if err := repository.Save(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, observation); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, observation); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duplicate error = %v", err)
	}
	if _, err := repository.List(nil, Scope{WorkspaceID: "workspace", UserID: "user"}, 10); err == nil {
		t.Fatal("nil context list unexpectedly accepted")
	}
	if _, err := repository.List(context.Background(), Scope{WorkspaceID: "", UserID: "user"}, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid list scope error = %v", err)
	}
	items, err := repository.List(context.Background(), Scope{WorkspaceID: "workspace", UserID: "user"}, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("stored observations = %#v, err=%v", items, err)
	}
}

func TestLearningValidationHelpersAndPostgresMapping(t *testing.T) {
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'x'
	}
	for _, value := range []string{"", "\n", string(long)} {
		if validIdentifier(value) {
			t.Fatalf("identifier %q unexpectedly valid", value)
		}
	}
	if !validIdentifier("workspace-1") {
		t.Fatal("normal identifier rejected")
	}
	if !errors.Is(mapPostgresError(pgx.ErrNoRows), ErrNotFound) {
		t.Fatal("no rows was not mapped")
	}
	marker := errors.New("marker")
	if !errors.Is(mapPostgresError(marker), marker) || mapPostgresError(nil) != nil {
		t.Fatal("non-database error mapping changed the error")
	}
	if _, err := NewPostgresRepository(nil); err == nil {
		t.Fatal("nil PostgreSQL pool unexpectedly accepted")
	}
}

func TestLearningValidationCoversRemainingInvalidFields(t *testing.T) {
	for _, scope := range []Scope{
		{WorkspaceID: "", UserID: "user"},
		{WorkspaceID: "workspace", UserID: ""},
	} {
		if err := scope.Validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid scope %#v error = %v", scope, err)
		}
	}
	for _, input := range []ObservationInput{
		{Prompt: "prompt", Response: "response"},
		{Skill: "skill", Response: "response"},
		{Skill: "skill", Prompt: "prompt"},
		{SourceID: "bad\nsource", Skill: "skill", Prompt: "prompt", Response: "response"},
	} {
		if err := input.validate(); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid input %#v error = %v", input, err)
		}
	}

	service, err := NewService(NewMemoryRepository())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(context.Background(), Scope{WorkspaceID: "", UserID: "user"}, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid list scope error = %v", err)
	}
}
