package contracts

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestTodayNormalizesVersionAndCollections(t *testing.T) {
	var decoded Today
	if err := json.Unmarshal([]byte(`{"date":"2026-08-26","projects":null,"tasks":null,"decisions":null}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != V1ContractVersion {
		t.Fatalf("version = %q, want %q", decoded.Version, V1ContractVersion)
	}
	if decoded.Projects == nil || decoded.Tasks == nil || decoded.Decisions == nil {
		t.Fatal("collections must be non-nil after decoding")
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" {
		t.Fatal("expected JSON output")
	}
}

func TestAssistantResponseKeepsGroundingFieldsExplicit(t *testing.T) {
	answer := AssistantResponse{Answer: "Use the documented timeout.", Unknowns: []string{"Provider latency is not recorded."}}
	encoded, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"evidence", "unknowns", "staleSources", "suggestedActions", "actionReceipts"} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("missing response field %q in %s", field, encoded)
		}
	}
	if got := fields["unknowns"].([]any); len(got) != 1 || got[0] != "Provider latency is not recorded." {
		t.Fatalf("unknowns = %#v", got)
	}
}

func TestContractRoundTripPreservesActionTargetAndTime(t *testing.T) {
	now := time.Date(2026, 8, 26, 8, 30, 45, 123, time.UTC)
	challenge := ActionChallenge{
		Version: V1ContractVersion, ID: "challenge-1", Action: "github.issue.create", TargetType: "repository", TargetID: "repo-1", ActionHash: "hash-1",
		Prompt: "Create the issue?", Parameters: JSONMap{"title": "Timeout regression"}, ExpiresAt: &now,
	}
	encoded, err := json.Marshal(challenge)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ActionChallenge
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != V1ContractVersion || decoded.TargetID != challenge.TargetID {
		t.Fatalf("decoded challenge = %+v", decoded)
	}
	if decoded.ExpiresAt == nil || !decoded.ExpiresAt.Equal(now) {
		t.Fatalf("expiresAt = %v, want %v", decoded.ExpiresAt, now)
	}

	receipt := ActionReceipt{ID: "receipt-1", ChallengeID: challenge.ID, IdempotencyKey: "request-1", Action: challenge.Action, TargetType: challenge.TargetType, TargetID: challenge.TargetID, Status: "accepted", Output: JSONMap{"issueId": "42"}, CreatedAt: &now}
	var decodedReceipt ActionReceipt
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &decodedReceipt); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedReceipt.Output, receipt.Output) {
		t.Fatalf("output = %#v, want %#v", decodedReceipt.Output, receipt.Output)
	}
}

func TestNestedContractsDefaultVersionAndMapsAcrossJSON(t *testing.T) {
	raw := []byte(`{"projects":[{"id":"p1","name":"Project"}],"tasks":[{"id":"t1","title":"Task","status":"todo"}],"decisions":[{"id":"d1","title":"Decision","decision":"Keep it"}]}`)
	var today Today
	if err := json.Unmarshal(raw, &today); err != nil {
		t.Fatal(err)
	}
	if today.Projects[0].Version != V1ContractVersion || today.Tasks[0].Version != V1ContractVersion || today.Decisions[0].Version != V1ContractVersion {
		t.Fatalf("nested versions were not normalized: %+v", today)
	}
	var challenge ActionChallenge
	if err := json.Unmarshal([]byte(`{"id":"c1","action":"create","prompt":"Create?","parameters":null}`), &challenge); err != nil {
		t.Fatal(err)
	}
	if challenge.Version != V1ContractVersion || challenge.Parameters == nil {
		t.Fatalf("challenge defaults = %+v", challenge)
	}
	encoded, err := json.Marshal(challenge)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || challenge.Parameters == nil {
		t.Fatal("challenge should serialize with an object parameters value")
	}
}

func TestRemainingContractsRoundTripAndNormalizeDefaults(t *testing.T) {
	now := time.Date(2026, 8, 26, 9, 15, 0, 0, time.UTC)
	description := "A project description"
	status := "active"
	repositoryURL := "https://github.com/example/project"
	projectID := "project-1"
	priority := "high"
	dueDate := "2026-09-01"
	context := "The current release is time-sensitive."
	rationale := "Keep the smaller change first."
	decisionStatus := "accepted"
	locator := "README.md:12"
	quote := "The timeout is bounded."
	score := 0.92
	message := "Action completed."

	project := roundTrip(t, Project{
		ID:            projectID,
		Name:          "DevEnglish",
		Description:   &description,
		Status:        &status,
		RepositoryURL: &repositoryURL,
		UpdatedAt:     &now,
	}, new(Project))
	if project.Version != V1ContractVersion || project.ID != projectID || project.UpdatedAt == nil {
		t.Fatalf("project defaults = %+v", project)
	}

	task := roundTrip(t, Task{
		ID:          "task-1",
		ProjectID:   &projectID,
		Title:       "Verify the timeout path",
		Description: &description,
		Status:      "todo",
		Priority:    &priority,
		DueDate:     &dueDate,
		UpdatedAt:   &now,
	}, new(Task))
	if task.Version != V1ContractVersion || task.ProjectID == nil || task.Priority == nil {
		t.Fatalf("task defaults = %+v", task)
	}

	decision := roundTrip(t, Decision{
		ID:        "decision-1",
		ProjectID: &projectID,
		Title:     "Keep the retry bounded",
		Decision:  "Use three retries",
		Context:   &context,
		Rationale: &rationale,
		Status:    &decisionStatus,
		DecidedAt: &now,
	}, new(Decision))
	if decision.Version != V1ContractVersion || decision.DecidedAt == nil || decision.Decision == "" {
		t.Fatalf("decision defaults = %+v", decision)
	}

	search := roundTrip(t, KnowledgeSearchResult{
		ID:        "chunk-1",
		Title:     "Timeout policy",
		Snippet:   "The client retries three times.",
		SourceURI: &repositoryURL,
		Score:     &score,
	}, new(KnowledgeSearchResult))
	if search.Version != V1ContractVersion || search.Evidence == nil || search.Score == nil {
		t.Fatalf("search defaults = %+v", search)
	}

	evidence := roundTrip(t, EvidenceRef{
		ID:        "evidence-1",
		SourceURI: repositoryURL,
		Locator:   &locator,
		Quote:     &quote,
	}, new(EvidenceRef))
	if evidence.Version != V1ContractVersion || evidence.Locator == nil || evidence.Quote == nil {
		t.Fatalf("evidence defaults = %+v", evidence)
	}

	grounded := roundTrip(t, GroundedAnswer{
		Answer:     "Use the bounded retry policy.",
		Confidence: &score,
	}, new(GroundedAnswer))
	if grounded.Version != V1ContractVersion || grounded.Evidence == nil || grounded.SearchResults == nil {
		t.Fatalf("grounded answer defaults = %+v", grounded)
	}

	suggested := roundTrip(t, SuggestedAction{
		Kind:              "github.issue.create",
		Title:             "Create a tracking issue",
		Description:       "Open a follow-up issue after confirmation.",
		NeedsConfirmation: true,
	}, new(SuggestedAction))
	if suggested.Version != V1ContractVersion || suggested.Parameters == nil || !suggested.NeedsConfirmation {
		t.Fatalf("suggested action defaults = %+v", suggested)
	}

	assistant := roundTrip(t, AssistantResponse{
		Answer: "The source does not record the deployment time.",
	}, new(AssistantResponse))
	if assistant.Version != V1ContractVersion || assistant.Evidence == nil || assistant.Unknowns == nil || assistant.StaleSources == nil || assistant.SuggestedActions == nil || assistant.ActionReceipts == nil {
		t.Fatalf("assistant response defaults = %+v", assistant)
	}

	receipt := roundTrip(t, ActionReceipt{
		ID:             "receipt-1",
		ChallengeID:    "challenge-1",
		IdempotencyKey: "request-1",
		Action:         "github.issue.create",
		TargetType:     "repository",
		TargetID:       "example/project",
		Status:         "completed",
		Message:        &message,
		CreatedAt:      &now,
	}, new(ActionReceipt))
	if receipt.Version != V1ContractVersion || receipt.Output == nil || receipt.Message == nil {
		t.Fatalf("receipt defaults = %+v", receipt)
	}
}

func roundTrip[T any](t *testing.T, value T, target *T) T {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("unmarshal %T: %v", value, err)
	}
	return *target
}
