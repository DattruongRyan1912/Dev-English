package assistant

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxIdentifierLength = 256
	maxTextLength       = 32 * 1024
)

// GroundingStatus is the trust label exposed to every assistant consumer.
// A response is never considered canonical merely because a model returned it.
type GroundingStatus string

const (
	Grounded GroundingStatus = "grounded"
	Inferred GroundingStatus = "inferred"
	Unknown  GroundingStatus = "unknown"
)

type EvidenceFreshness string

const (
	FreshCurrent EvidenceFreshness = "current"
	FreshStale   EvidenceFreshness = "stale"
	FreshUnknown EvidenceFreshness = "unknown"
)

// Scope is the authenticated application boundary for every assistant
// request and every piece of retrieved evidence. It is intentionally local to
// this package so REST and MCP adapters must map their authenticated principal
// into an explicit scope before calling the service.
type Scope struct {
	WorkspaceID string `json:"workspaceId"`
	UserID      string `json:"userId"`
}

// UnknownAnswer is the deterministic fail-closed answer used when the model
// supplied no verifiable citation. The original ungrounded model answer is
// intentionally not returned to the caller.
const UnknownAnswer = "I don't have enough verified evidence to answer that yet."

// Evidence is retrieval output, not a canonical claim. Its freshness must be
// explicit so stale or incomplete source state cannot silently look current.
type Evidence struct {
	Scope      Scope `json:"scope"`
	ID         string
	SourceID   string
	RevisionID string
	Title      string
	URI        string
	Snippet    string
	Freshness  EvidenceFreshness
}

// Citation is the only way a generated answer can refer to retrieved data.
// EvidenceID must match an item supplied by the retriever; the adapter never
// accepts an arbitrary URL or source string from the model as proof.
type Citation struct {
	EvidenceID string `json:"evidenceId"`
	Quote      string `json:"quote,omitempty"`
	Locator    string `json:"locator,omitempty"`
}

// SuggestedAction is declarative data. It contains no executable callback or
// tool invocation; a later application-service layer must challenge and
// authorize any mutation independently.
type SuggestedAction struct {
	ID                   string `json:"id"`
	Kind                 string `json:"kind"`
	Label                string `json:"label"`
	Target               string `json:"target,omitempty"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
}

const (
	ReceiptPending   = "pending"
	ReceiptUncertain = "uncertain"
	ReceiptAccepted  = "accepted"
)

// ActionReceipt is the assistant-facing projection of a trusted application
// action-service receipt. It intentionally is not part of Draft: a model
// cannot manufacture a receipt through the provider response path. The
// identity fields mirror the Wave 2 safe-write receipt contract so a transport
// adapter cannot attach a receipt to a different user, challenge or mutation.
type ActionReceipt struct {
	ID             string `json:"id"`
	ActionID       string `json:"actionId"`
	Scope          Scope  `json:"scope"`
	Provider       string `json:"provider"`
	Operation      string `json:"operation"`
	ChallengeID    string `json:"challengeId"`
	IdempotencyKey string `json:"idempotencyKey"`
	ActionHash     string `json:"actionHash"`
	TargetType     string `json:"targetType"`
	TargetID       string `json:"targetId"`
	Status         string `json:"status"`
	Replayed       bool   `json:"replayed,omitempty"`
}

// ActionBinding is the canonical identity returned by the trusted application
// action service after it has resolved and challenged a mutation. It is kept
// separate from SuggestedAction because model output is untrusted and must not
// define the operation, target or action hash used for receipt verification.
type ActionBinding struct {
	ActionID       string `json:"actionId"`
	Scope          Scope  `json:"scope"`
	Provider       string `json:"provider"`
	Operation      string `json:"operation"`
	ChallengeID    string `json:"challengeId"`
	IdempotencyKey string `json:"idempotencyKey"`
	ActionHash     string `json:"actionHash"`
	TargetType     string `json:"targetType"`
	TargetID       string `json:"targetId"`
}

// ActionReceiptAttachment is the only input accepted by AttachActionReceipts.
// The unexported trusted marker can only be set by the constructor, so a REST,
// MCP or model-facing payload cannot directly inject a receipt attachment.
type ActionReceiptAttachment struct {
	Binding ActionBinding
	Receipt ActionReceipt
	trusted bool
}

// NewActionReceiptAttachment validates a canonical binding and its receipt as
// one identity. The real action service should call this only after challenge
// confirmation or replay resolution from its canonical store.
func NewActionReceiptAttachment(binding ActionBinding, receipt ActionReceipt) (ActionReceiptAttachment, error) {
	if err := validateActionBindingAndReceipt(binding, receipt, binding.Scope); err != nil {
		return ActionReceiptAttachment{}, err
	}
	return ActionReceiptAttachment{Binding: binding, Receipt: receipt, trusted: true}, nil
}

// Draft is the provider-facing result before the grounding boundary runs.
// Draft data is untrusted and must not be sent directly to a user or tool.
type Draft struct {
	Answer           string
	Citations        []Citation
	Unknowns         []string
	SuggestedActions []SuggestedAction
}

// AssistantResponse is the stable application/MCP contract for grounded
// assistant responses.
type AssistantResponse struct {
	Scope            Scope             `json:"scope"`
	Answer           string            `json:"answer"`
	Grounding        GroundingStatus   `json:"grounding"`
	Evidence         []Citation        `json:"evidence"`
	Unknowns         []string          `json:"unknowns"`
	StaleSources     []string          `json:"staleSources"`
	SuggestedActions []SuggestedAction `json:"suggestedActions"`
	ActionReceipts   []ActionReceipt   `json:"actionReceipts"`
}

type AskRequest struct {
	Scope          Scope
	ConversationID string
	Message        string
}

type RetrievalRequest struct {
	Scope          Scope
	ConversationID string
	Query          string
}

type GenerationRequest struct {
	Scope          Scope
	ConversationID string
	Message        string
	Evidence       []Evidence
}

type Retriever interface {
	Search(context.Context, RetrievalRequest) ([]Evidence, error)
}

type Generator interface {
	Generate(context.Context, GenerationRequest) (Draft, error)
}

// Service is the provider-neutral application boundary for assistant asks.
// Persistence and provider routing are intentionally injected later; this
// first slice makes grounding deterministic before any transport is attached.
type Service struct {
	retriever Retriever
	generator Generator
}

func NewService(retriever Retriever, generator Generator) (*Service, error) {
	if retriever == nil {
		return nil, ErrNilRetriever
	}
	if generator == nil {
		return nil, ErrNilGenerator
	}
	return &Service{retriever: retriever, generator: generator}, nil
}

func (s *Service) Ask(ctx context.Context, request AskRequest) (AssistantResponse, error) {
	if s == nil || s.retriever == nil || s.generator == nil {
		return AssistantResponse{}, ErrInvalidInput
	}
	if ctx == nil {
		return AssistantResponse{}, invalidField("context", "must not be nil")
	}
	if err := validateScope("scope", request.Scope); err != nil {
		return AssistantResponse{}, err
	}
	if err := validateRequiredText("message", request.Message); err != nil {
		return AssistantResponse{}, err
	}
	request.Message = strings.TrimSpace(request.Message)
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	if err := validateOptionalIdentifier("conversationId", request.ConversationID); err != nil {
		return AssistantResponse{}, err
	}

	evidence, err := s.retriever.Search(ctx, RetrievalRequest{
		Scope:          request.Scope,
		ConversationID: request.ConversationID,
		Query:          request.Message,
	})
	if err != nil {
		return AssistantResponse{}, dependencyFailure("retrieval", err)
	}
	prepared, err := prepareEvidence(request.Scope, evidence)
	if err != nil {
		return AssistantResponse{}, err
	}
	if len(prepared) == 0 {
		return unknownResponse(request.Scope, []string{"No verified evidence was retrieved."}), nil
	}
	draft, err := s.generator.Generate(ctx, GenerationRequest{
		Scope:          request.Scope,
		ConversationID: request.ConversationID,
		Message:        request.Message,
		Evidence:       cloneEvidence(prepared),
	})
	if err != nil {
		return AssistantResponse{}, dependencyFailure("generation", err)
	}
	return NormalizeResponse(request.Scope, draft, prepared)
}

// NormalizeResponse applies the fail-closed grounding and action boundary.
// It is deterministic and has no provider, network, database or tool side
// effects, which makes it safe to reuse from REST and MCP adapters.
func NormalizeResponse(scope Scope, draft Draft, available []Evidence) (AssistantResponse, error) {
	if err := validateScope("scope", scope); err != nil {
		return AssistantResponse{}, err
	}
	if err := validateRequiredText("answer", draft.Answer); err != nil {
		return AssistantResponse{}, err
	}
	prepared, err := prepareEvidence(scope, available)
	if err != nil {
		return AssistantResponse{}, err
	}
	byID := make(map[string]Evidence, len(prepared))
	for _, item := range prepared {
		byID[item.ID] = item
	}

	citations, staleSources, hasStale, err := normalizeCitations(draft.Citations, byID, draft.Answer)
	if err != nil {
		return AssistantResponse{}, err
	}
	actions, err := normalizeActions(draft.SuggestedActions)
	if err != nil {
		return AssistantResponse{}, err
	}
	unknowns, err := normalizeTextList("unknowns", draft.Unknowns)
	if err != nil {
		return AssistantResponse{}, err
	}
	if len(citations) == 0 {
		return unknownResponse(scope, appendUnique(unknowns, "No matching evidence was cited for this answer.")), nil
	}

	grounding := Grounded
	if hasStale {
		grounding = Inferred
		unknowns = appendUnique(unknowns, "The answer relies on evidence that is not confirmed current.")
	}
	return AssistantResponse{
		Scope:            scope,
		Answer:           strings.TrimSpace(draft.Answer),
		Grounding:        grounding,
		Evidence:         citations,
		Unknowns:         unknowns,
		StaleSources:     staleSources,
		SuggestedActions: actions,
		ActionReceipts:   []ActionReceipt{},
	}, nil
}

// AttachActionReceipts appends receipts produced by a trusted application
// action service. A binding must refer to an action in the response and is
// validated here before it reaches REST, MCP or the UI.
func AttachActionReceipts(response AssistantResponse, input []ActionReceiptAttachment) (AssistantResponse, error) {
	if err := validateScope("scope", response.Scope); err != nil {
		return AssistantResponse{}, err
	}
	receipts, err := normalizeReceiptAttachments(input, response.Scope, response.SuggestedActions)
	if err != nil {
		return AssistantResponse{}, err
	}
	response.Evidence = append([]Citation(nil), response.Evidence...)
	response.Unknowns = append([]string(nil), response.Unknowns...)
	response.StaleSources = append([]string(nil), response.StaleSources...)
	response.SuggestedActions = append([]SuggestedAction(nil), response.SuggestedActions...)
	response.ActionReceipts = receipts
	return response, nil
}

func unknownResponse(scope Scope, unknowns []string) AssistantResponse {
	return AssistantResponse{
		Scope:            scope,
		Answer:           UnknownAnswer,
		Grounding:        Unknown,
		Evidence:         []Citation{},
		Unknowns:         append([]string{}, unknowns...),
		StaleSources:     []string{},
		SuggestedActions: []SuggestedAction{},
		ActionReceipts:   []ActionReceipt{},
	}
}

func dependencyFailure(stage string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return fmt.Errorf("%w: %s", ErrDependencyFailure, stage)
}

func prepareEvidence(scope Scope, evidence []Evidence) ([]Evidence, error) {
	if err := validateScope("scope", scope); err != nil {
		return nil, err
	}
	prepared := make([]Evidence, 0, len(evidence))
	seen := make(map[string]struct{}, len(evidence))
	for index, item := range evidence {
		prefix := "evidence[" + strconv.Itoa(index) + "]"
		if err := validateScope(prefix+".scope", item.Scope); err != nil {
			return nil, err
		}
		if !sameScope(item.Scope, scope) {
			return nil, fmt.Errorf("%w: %s", ErrScopeMismatch, prefix+".scope")
		}
		if err := validateIdentifier(prefix+".id", item.ID); err != nil {
			return nil, err
		}
		if _, exists := seen[item.ID]; exists {
			return nil, ErrDuplicateEvidence
		}
		if item.Freshness != FreshCurrent && item.Freshness != FreshStale && item.Freshness != FreshUnknown {
			return nil, invalidField(prefix+".freshness", "must be current, stale, or unknown")
		}
		if err := validateRequiredText(prefix+".snippet", item.Snippet); err != nil {
			return nil, err
		}
		if err := validateOptionalText(prefix+".sourceId", item.SourceID); err != nil {
			return nil, err
		}
		if err := validateOptionalText(prefix+".revisionId", item.RevisionID); err != nil {
			return nil, err
		}
		if err := validateOptionalText(prefix+".title", item.Title); err != nil {
			return nil, err
		}
		if err := validateOptionalText(prefix+".uri", item.URI); err != nil {
			return nil, err
		}
		seen[item.ID] = struct{}{}
		item.Scope.WorkspaceID = strings.TrimSpace(item.Scope.WorkspaceID)
		item.Scope.UserID = strings.TrimSpace(item.Scope.UserID)
		item.Title = strings.TrimSpace(item.Title)
		item.URI = strings.TrimSpace(item.URI)
		item.Snippet = strings.TrimSpace(item.Snippet)
		prepared = append(prepared, item)
	}
	return prepared, nil
}

func normalizeCitations(input []Citation, evidence map[string]Evidence, answer string) ([]Citation, []string, bool, error) {
	result := make([]Citation, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	stale := make(map[string]struct{})
	hasStale := false
	for index, citation := range input {
		if err := validateIdentifier("evidence["+strconv.Itoa(index)+"].evidenceId", citation.EvidenceID); err != nil {
			return nil, nil, false, err
		}
		if _, exists := seen[citation.EvidenceID]; exists {
			return nil, nil, false, ErrDuplicateCitation
		}
		item, exists := evidence[citation.EvidenceID]
		if !exists {
			return nil, nil, false, ErrEvidenceNotFound
		}
		citation.Quote = strings.TrimSpace(citation.Quote)
		citation.Locator = strings.TrimSpace(citation.Locator)
		if citation.Quote == "" {
			return nil, nil, false, ErrQuoteRequired
		}
		if err := validateRequiredText("evidence["+strconv.Itoa(index)+"].quote", citation.Quote); err != nil {
			return nil, nil, false, err
		}
		if err := validateOptionalText("evidence["+strconv.Itoa(index)+"].locator", citation.Locator); err != nil {
			return nil, nil, false, err
		}
		if !strings.Contains(item.Snippet, citation.Quote) {
			return nil, nil, false, ErrQuoteNotSupported
		}
		if !strings.Contains(strings.TrimSpace(answer), citation.Quote) {
			return nil, nil, false, ErrQuoteNotInAnswer
		}
		seen[citation.EvidenceID] = struct{}{}
		result = append(result, citation)
		if item.Freshness != FreshCurrent {
			hasStale = true
			stale[staleKey(item)] = struct{}{}
		}
	}
	staleSources := make([]string, 0, len(stale))
	for source := range stale {
		staleSources = append(staleSources, source)
	}
	sort.Strings(staleSources)
	return result, staleSources, hasStale, nil
}

func normalizeActions(input []SuggestedAction) ([]SuggestedAction, error) {
	result := make([]SuggestedAction, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for index, action := range input {
		action.ID = strings.TrimSpace(action.ID)
		action.Kind = strings.TrimSpace(action.Kind)
		action.Label = strings.TrimSpace(action.Label)
		action.Target = strings.TrimSpace(action.Target)
		prefix := "suggestedActions[" + strconv.Itoa(index) + "]"
		if err := validateIdentifier(prefix+".id", action.ID); err != nil {
			return nil, invalidAction(prefix+".id", "must be a valid action identifier")
		}
		if _, exists := seen[action.ID]; exists {
			return nil, invalidAction(prefix+".id", "must be unique")
		}
		if err := validateRequiredText(prefix+".kind", action.Kind); err != nil {
			return nil, invalidAction(prefix+".kind", "must not be blank")
		}
		if err := validateRequiredText(prefix+".label", action.Label); err != nil {
			return nil, invalidAction(prefix+".label", "must not be blank")
		}
		if err := validateOptionalText(prefix+".target", action.Target); err != nil {
			return nil, invalidAction(prefix+".target", "must be bounded text")
		}
		seen[action.ID] = struct{}{}
		// Every action is confirmation-gated at this boundary, even if an
		// untrusted provider forgot to set the flag.
		action.RequiresConfirmation = true
		result = append(result, action)
	}
	return result, nil
}

func normalizeReceiptAttachments(input []ActionReceiptAttachment, responseScope Scope, actions []SuggestedAction) ([]ActionReceipt, error) {
	result := make([]ActionReceipt, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	knownActions := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		knownActions[action.ID] = struct{}{}
	}
	for index, attachment := range input {
		prefix := "actionReceipts[" + strconv.Itoa(index) + "]"
		if !attachment.trusted {
			return nil, ErrUntrustedReceipt
		}
		if err := validateActionBindingAndReceipt(attachment.Binding, attachment.Receipt, responseScope); err != nil {
			return nil, err
		}
		receipt := attachment.Receipt
		if err := validateIdentifier(prefix+".id", receipt.ID); err != nil {
			return nil, invalidReceipt(prefix+".id", "must be a valid receipt identifier")
		}
		if _, exists := seen[receipt.ID]; exists {
			return nil, invalidReceipt(prefix+".id", "must be unique")
		}
		if _, exists := knownActions[attachment.Binding.ActionID]; !exists {
			return nil, ErrReceiptActionMissing
		}
		seen[receipt.ID] = struct{}{}
		result = append(result, receipt)
	}
	return result, nil
}

func validateActionBindingAndReceipt(binding ActionBinding, receipt ActionReceipt, responseScope Scope) error {
	if err := validateScope("actionBinding.scope", binding.Scope); err != nil {
		return err
	}
	if !sameScope(binding.Scope, responseScope) {
		return ErrScopeMismatch
	}
	if err := validateScope("actionReceipt.scope", receipt.Scope); err != nil {
		return invalidReceipt("actionReceipt.scope", "must contain workspace and user scope")
	}
	if !sameScope(receipt.Scope, responseScope) {
		return ErrScopeMismatch
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"actionId", binding.ActionID},
		{"provider", binding.Provider},
		{"operation", binding.Operation},
		{"challengeId", binding.ChallengeID},
		{"idempotencyKey", binding.IdempotencyKey},
		{"actionHash", binding.ActionHash},
		{"targetType", binding.TargetType},
		{"targetId", binding.TargetID},
	} {
		if err := validateIdentifier("actionBinding."+field.name, field.value); err != nil {
			return err
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"actionId", receipt.ActionID},
		{"provider", receipt.Provider},
		{"operation", receipt.Operation},
		{"challengeId", receipt.ChallengeID},
		{"idempotencyKey", receipt.IdempotencyKey},
		{"actionHash", receipt.ActionHash},
		{"targetType", receipt.TargetType},
		{"targetId", receipt.TargetID},
	} {
		if err := validateIdentifier("actionReceipt."+field.name, field.value); err != nil {
			return invalidReceipt("actionReceipt."+field.name, "must be present and bounded")
		}
	}
	if binding.ActionID != receipt.ActionID ||
		binding.Scope != receipt.Scope ||
		binding.Provider != receipt.Provider ||
		binding.Operation != receipt.Operation ||
		binding.ChallengeID != receipt.ChallengeID ||
		binding.IdempotencyKey != receipt.IdempotencyKey ||
		binding.ActionHash != receipt.ActionHash ||
		binding.TargetType != receipt.TargetType ||
		binding.TargetID != receipt.TargetID {
		return ErrReceiptBindingMismatch
	}
	switch receipt.Status {
	case ReceiptPending, ReceiptUncertain, ReceiptAccepted:
	default:
		return invalidReceipt("actionReceipt.status", "must be pending, uncertain, or accepted")
	}
	if receipt.Replayed && receipt.Status != ReceiptAccepted {
		return invalidReceipt("actionReceipt.replayed", "may only be true for an accepted receipt")
	}
	return nil
}

func validateIdentifier(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalidField(field, "must not be blank")
	}
	if strings.TrimSpace(value) != value {
		return invalidField(field, "must not have leading or trailing whitespace")
	}
	if len(value) > maxIdentifierLength {
		return invalidField(field, "exceeds maximum length")
	}
	if hasDisallowedControl(value, false) {
		return invalidField(field, "must not contain control characters")
	}
	return nil
}

func validateRequiredText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return invalidField(field, "must not be blank")
	}
	if len(value) > maxTextLength {
		return invalidField(field, "exceeds maximum length")
	}
	if hasDisallowedControl(value, true) {
		return invalidField(field, "must not contain disallowed control characters")
	}
	return nil
}

func validateOptionalText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateRequiredText(field, value)
}

func validateOptionalIdentifier(field, value string) error {
	if value == "" {
		return nil
	}
	return validateIdentifier(field, value)
}

func validateScope(field string, scope Scope) error {
	if err := validateIdentifier(field+".workspaceId", scope.WorkspaceID); err != nil {
		return err
	}
	if err := validateIdentifier(field+".userId", scope.UserID); err != nil {
		return err
	}
	return nil
}

func sameScope(left, right Scope) bool {
	return left.WorkspaceID == right.WorkspaceID && left.UserID == right.UserID
}

func hasDisallowedControl(value string, allowTextControls bool) bool {
	for _, r := range value {
		if !unicode.IsControl(r) {
			continue
		}
		if allowTextControls && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		return true
	}
	return false
}

func normalizeTextList(field string, input []string) ([]string, error) {
	result := make([]string, 0, len(input))
	for index, value := range input {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := validateRequiredText(field+"["+strconv.Itoa(index)+"]", value); err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		result = appendUnique(result, value)
	}
	return result, nil
}

func staleKey(item Evidence) string {
	if item.SourceID != "" {
		return item.SourceID
	}
	return item.ID
}

func cleanTextList(input []string) []string {
	result := make([]string, 0, len(input))
	for _, value := range input {
		value = strings.TrimSpace(value)
		if value != "" {
			result = appendUnique(result, value)
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func cloneEvidence(input []Evidence) []Evidence {
	result := make([]Evidence, len(input))
	copy(result, input)
	return result
}
