// Package contracts contains transport-only V1 contracts shared by the
// backend and Flutter client. These types do not depend on HTTP or domain
// implementations, so REST and MCP can reuse the same JSON semantics.
package contracts

import (
	"encoding/json"
	"time"
)

// V1ContractVersion is the version value used by the initial contract set.
const V1ContractVersion = "v1"

// JSONMap is an opaque JSON object used for action input/output payloads.
type JSONMap map[string]any

// Today is the read-only daily workspace snapshot.
type Today struct {
	Version     string     `json:"version"`
	Date        string     `json:"date"`
	Summary     *string    `json:"summary"`
	FocusTaskID *string    `json:"focusTaskId"`
	Projects    []Project  `json:"projects"`
	Tasks       []Task     `json:"tasks"`
	Decisions   []Decision `json:"decisions"`
}

func (v Today) MarshalJSON() ([]byte, error) {
	type plain Today
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Projects = nonNil(value.Projects)
	value.Tasks = nonNil(value.Tasks)
	value.Decisions = nonNil(value.Decisions)
	return json.Marshal(value)
}

func (v *Today) UnmarshalJSON(data []byte) error {
	type plain Today
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Projects = nonNil(value.Projects)
	value.Tasks = nonNil(value.Tasks)
	value.Decisions = nonNil(value.Decisions)
	*v = Today(value)
	return nil
}

// Project identifies a project and its current display metadata.
type Project struct {
	Version       string     `json:"version"`
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   *string    `json:"description"`
	Status        *string    `json:"status"`
	RepositoryURL *string    `json:"repositoryUrl"`
	UpdatedAt     *time.Time `json:"updatedAt"`
}

func (v Project) MarshalJSON() ([]byte, error) {
	type plain Project
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	return json.Marshal(value)
}

func (v *Project) UnmarshalJSON(data []byte) error {
	type plain Project
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	*v = Project(value)
	return nil
}

// Task is a transport representation of a project task. DueDate is YYYY-MM-DD.
type Task struct {
	Version     string     `json:"version"`
	ID          string     `json:"id"`
	ProjectID   *string    `json:"projectId"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Status      string     `json:"status"`
	Priority    *string    `json:"priority"`
	DueDate     *string    `json:"dueDate"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}

func (v Task) MarshalJSON() ([]byte, error) {
	type plain Task
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	return json.Marshal(value)
}

func (v *Task) UnmarshalJSON(data []byte) error {
	type plain Task
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	*v = Task(value)
	return nil
}

// Decision records an explicit project decision without making its derived
// rationale a canonical knowledge claim.
type Decision struct {
	Version   string     `json:"version"`
	ID        string     `json:"id"`
	ProjectID *string    `json:"projectId"`
	Title     string     `json:"title"`
	Decision  string     `json:"decision"`
	Context   *string    `json:"context"`
	Rationale *string    `json:"rationale"`
	Status    *string    `json:"status"`
	DecidedAt *time.Time `json:"decidedAt"`
}

func (v Decision) MarshalJSON() ([]byte, error) {
	type plain Decision
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	return json.Marshal(value)
}

func (v *Decision) UnmarshalJSON(data []byte) error {
	type plain Decision
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	*v = Decision(value)
	return nil
}

// KnowledgeSearchResult is one ranked, source-addressable knowledge hit.
type KnowledgeSearchResult struct {
	Version   string        `json:"version"`
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Snippet   string        `json:"snippet"`
	SourceURI *string       `json:"sourceUri"`
	Score     *float64      `json:"score"`
	Evidence  []EvidenceRef `json:"evidence"`
}

func (v KnowledgeSearchResult) MarshalJSON() ([]byte, error) {
	type plain KnowledgeSearchResult
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Evidence = nonNil(value.Evidence)
	return json.Marshal(value)
}

func (v *KnowledgeSearchResult) UnmarshalJSON(data []byte) error {
	type plain KnowledgeSearchResult
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Evidence = nonNil(value.Evidence)
	*v = KnowledgeSearchResult(value)
	return nil
}

// EvidenceRef points to the source location supporting a result or answer.
type EvidenceRef struct {
	Version   string  `json:"version"`
	ID        string  `json:"id"`
	SourceURI string  `json:"sourceUri"`
	Locator   *string `json:"locator"`
	Quote     *string `json:"quote"`
}

func (v EvidenceRef) MarshalJSON() ([]byte, error) {
	type plain EvidenceRef
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	return json.Marshal(value)
}

func (v *EvidenceRef) UnmarshalJSON(data []byte) error {
	type plain EvidenceRef
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	*v = EvidenceRef(value)
	return nil
}

// GroundedAnswer is an answer plus the evidence and search hits used to ground
// it. It does not perform retrieval or claim verification.
type GroundedAnswer struct {
	Version       string                  `json:"version"`
	Answer        string                  `json:"answer"`
	Confidence    *float64                `json:"confidence"`
	Evidence      []EvidenceRef           `json:"evidence"`
	SearchResults []KnowledgeSearchResult `json:"searchResults"`
}

func (v GroundedAnswer) MarshalJSON() ([]byte, error) {
	type plain GroundedAnswer
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Evidence = nonNil(value.Evidence)
	value.SearchResults = nonNil(value.SearchResults)
	return json.Marshal(value)
}

func (v *GroundedAnswer) UnmarshalJSON(data []byte) error {
	type plain GroundedAnswer
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Evidence = nonNil(value.Evidence)
	value.SearchResults = nonNil(value.SearchResults)
	*v = GroundedAnswer(value)
	return nil
}

// SuggestedAction is a non-executing recommendation from the assistant.
type SuggestedAction struct {
	Version           string  `json:"version"`
	Kind              string  `json:"kind"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	NeedsConfirmation bool    `json:"needsConfirmation"`
	Parameters        JSONMap `json:"parameters"`
}

func (v SuggestedAction) MarshalJSON() ([]byte, error) {
	type plain SuggestedAction
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Parameters = nonNilMap(value.Parameters)
	return json.Marshal(value)
}

func (v *SuggestedAction) UnmarshalJSON(data []byte) error {
	type plain SuggestedAction
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Parameters = nonNilMap(value.Parameters)
	*v = SuggestedAction(value)
	return nil
}

// AssistantResponse is the stable response envelope for grounded assistant
// calls. Unknowns and stale sources are explicit so an empty evidence list is
// never silently treated as a verified answer.
type AssistantResponse struct {
	Version          string            `json:"version"`
	Answer           string            `json:"answer"`
	Evidence         []EvidenceRef     `json:"evidence"`
	Unknowns         []string          `json:"unknowns"`
	StaleSources     []EvidenceRef     `json:"staleSources"`
	SuggestedActions []SuggestedAction `json:"suggestedActions"`
	ActionReceipts   []ActionReceipt   `json:"actionReceipts"`
}

func (v AssistantResponse) MarshalJSON() ([]byte, error) {
	type plain AssistantResponse
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Evidence = nonNil(value.Evidence)
	value.Unknowns = nonNil(value.Unknowns)
	value.StaleSources = nonNil(value.StaleSources)
	value.SuggestedActions = nonNil(value.SuggestedActions)
	value.ActionReceipts = nonNil(value.ActionReceipts)
	return json.Marshal(value)
}

func (v *AssistantResponse) UnmarshalJSON(data []byte) error {
	type plain AssistantResponse
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Evidence = nonNil(value.Evidence)
	value.Unknowns = nonNil(value.Unknowns)
	value.StaleSources = nonNil(value.StaleSources)
	value.SuggestedActions = nonNil(value.SuggestedActions)
	value.ActionReceipts = nonNil(value.ActionReceipts)
	*v = AssistantResponse(value)
	return nil
}

// ActionChallenge describes an action presented for explicit confirmation by
// a separate action-safety layer. It does not authorize or execute the action.
type ActionChallenge struct {
	Version    string     `json:"version"`
	ID         string     `json:"id"`
	Action     string     `json:"action"`
	TargetType string     `json:"targetType"`
	TargetID   string     `json:"targetId"`
	ActionHash string     `json:"actionHash"`
	Prompt     string     `json:"prompt"`
	Parameters JSONMap    `json:"parameters"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

func (v ActionChallenge) MarshalJSON() ([]byte, error) {
	type plain ActionChallenge
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Parameters = nonNilMap(value.Parameters)
	return json.Marshal(value)
}

func (v *ActionChallenge) UnmarshalJSON(data []byte) error {
	type plain ActionChallenge
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Parameters = nonNilMap(value.Parameters)
	*v = ActionChallenge(value)
	return nil
}

// ActionReceipt is the transport result of an action attempt. A receipt is
// evidence of the action result, not permission to execute another action.
type ActionReceipt struct {
	Version        string     `json:"version"`
	ID             string     `json:"id"`
	ChallengeID    string     `json:"challengeId"`
	IdempotencyKey string     `json:"idempotencyKey"`
	Action         string     `json:"action"`
	TargetType     string     `json:"targetType"`
	TargetID       string     `json:"targetId"`
	Status         string     `json:"status"`
	Message        *string    `json:"message"`
	Output         JSONMap    `json:"output"`
	CreatedAt      *time.Time `json:"createdAt"`
}

func (v ActionReceipt) MarshalJSON() ([]byte, error) {
	type plain ActionReceipt
	value := plain(v)
	value.Version = versionOrDefault(value.Version)
	value.Output = nonNilMap(value.Output)
	return json.Marshal(value)
}

func (v *ActionReceipt) UnmarshalJSON(data []byte) error {
	type plain ActionReceipt
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	value.Version = versionOrDefault(value.Version)
	value.Output = nonNilMap(value.Output)
	*v = ActionReceipt(value)
	return nil
}

func versionOrDefault(value string) string {
	if value == "" {
		return V1ContractVersion
	}
	return value
}

func nonNil[T any](value []T) []T {
	if value == nil {
		return []T{}
	}
	return value
}

func nonNilMap(value JSONMap) JSONMap {
	if value == nil {
		return JSONMap{}
	}
	return value
}
