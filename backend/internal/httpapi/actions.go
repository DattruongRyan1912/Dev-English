package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/actions"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/mcp"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

// actionTargetRequest is deliberately a flat, transport-facing projection.
// It contains only the canonical action target; credentials and provider
// responses never cross this boundary.
type actionTargetRequest struct {
	Operation        string   `json:"operation"`
	EntityType       string   `json:"entityType"`
	EntityID         string   `json:"entityId"`
	ExpectedVersion  int64    `json:"expectedVersion"`
	Repository       string   `json:"repository"`
	Issue            int64    `json:"issue"`
	Title            string   `json:"title"`
	Body             string   `json:"body"`
	Labels           []string `json:"labels"`
	ExpectedRevision string   `json:"expectedRevision"`
}

func (request actionTargetRequest) target() connectors.SafeWriteTarget {
	return connectors.SafeWriteTarget{
		Operation:        connectors.SafeWriteOperation(strings.TrimSpace(request.Operation)),
		EntityType:       strings.TrimSpace(request.EntityType),
		EntityID:         strings.TrimSpace(request.EntityID),
		ExpectedVersion:  request.ExpectedVersion,
		Repository:       request.Repository,
		Issue:            request.Issue,
		Title:            request.Title,
		Body:             request.Body,
		Labels:           append([]string(nil), request.Labels...),
		ExpectedRevision: request.ExpectedRevision,
	}
}

type createActionChallengeRequest struct {
	Operation        string   `json:"operation"`
	EntityType       string   `json:"entityType"`
	EntityID         string   `json:"entityId"`
	ExpectedVersion  int64    `json:"expectedVersion"`
	Repository       string   `json:"repository"`
	Issue            int64    `json:"issue"`
	Title            string   `json:"title"`
	Body             string   `json:"body"`
	Labels           []string `json:"labels"`
	ExpectedRevision string   `json:"expectedRevision"`
	IdempotencyKey   string   `json:"idempotencyKey"`
}

func (request createActionChallengeRequest) target() connectors.SafeWriteTarget {
	return actionTargetRequest{
		Operation:        request.Operation,
		EntityType:       request.EntityType,
		EntityID:         request.EntityID,
		ExpectedVersion:  request.ExpectedVersion,
		Repository:       request.Repository,
		Issue:            request.Issue,
		Title:            request.Title,
		Body:             request.Body,
		Labels:           request.Labels,
		ExpectedRevision: request.ExpectedRevision,
	}.target()
}

type confirmActionRequest struct {
	ChallengeID      string   `json:"challengeId"`
	Operation        string   `json:"operation"`
	EntityType       string   `json:"entityType"`
	EntityID         string   `json:"entityId"`
	ExpectedVersion  int64    `json:"expectedVersion"`
	Repository       string   `json:"repository"`
	Issue            int64    `json:"issue"`
	Title            string   `json:"title"`
	Body             string   `json:"body"`
	Labels           []string `json:"labels"`
	ExpectedRevision string   `json:"expectedRevision"`
	IdempotencyKey   string   `json:"idempotencyKey"`
}

func (request confirmActionRequest) target() connectors.SafeWriteTarget {
	return actionTargetRequest{
		Operation:        request.Operation,
		EntityType:       request.EntityType,
		EntityID:         request.EntityID,
		ExpectedVersion:  request.ExpectedVersion,
		Repository:       request.Repository,
		Issue:            request.Issue,
		Title:            request.Title,
		Body:             request.Body,
		Labels:           request.Labels,
		ExpectedRevision: request.ExpectedRevision,
	}.target()
}

type actionChallengeResponse struct {
	Action    assistant.ActionBinding `json:"action"`
	Target    actionTargetResponse    `json:"target"`
	ExpiresAt time.Time               `json:"expiresAt"`
}

type actionTargetResponse struct {
	Operation        connectors.SafeWriteOperation `json:"operation"`
	EntityType       string                        `json:"entityType,omitempty"`
	EntityID         string                        `json:"entityId,omitempty"`
	ExpectedVersion  int64                         `json:"expectedVersion,omitempty"`
	Repository       string                        `json:"repository"`
	Issue            int64                         `json:"issue,omitempty"`
	Title            string                        `json:"title,omitempty"`
	Body             string                        `json:"body,omitempty"`
	Labels           []string                      `json:"labels,omitempty"`
	ExpectedRevision string                        `json:"expectedRevision,omitempty"`
}

func newActionTargetResponse(target connectors.SafeWriteTarget) actionTargetResponse {
	return actionTargetResponse{
		Operation:        target.Operation,
		EntityType:       strings.TrimSpace(target.EntityType),
		EntityID:         strings.TrimSpace(target.EntityID),
		ExpectedVersion:  target.ExpectedVersion,
		Repository:       strings.TrimSpace(target.Repository),
		Issue:            target.Issue,
		Title:            target.Title,
		Body:             target.Body,
		Labels:           append([]string(nil), target.Labels...),
		ExpectedRevision: strings.TrimSpace(target.ExpectedRevision),
	}
}

type actionConfirmationResponse struct {
	Action   assistant.ActionBinding `json:"action"`
	Receipt  assistant.ActionReceipt `json:"receipt"`
	Replayed bool                    `json:"replayed"`
}

type mcpTokenIssueRequest struct {
	Scopes []mcp.Scope `json:"scopes"`
}

type mcpTokenResponse struct {
	ID        string      `json:"id"`
	Token     string      `json:"token"`
	Scopes    []mcp.Scope `json:"scopes"`
	ExpiresAt time.Time   `json:"expiresAt"`
}

var (
	errInvalidActionIdempotencyKey = errors.New("invalid action idempotency key")
	errActionIdempotencyRequired   = errors.New("action idempotency key is required")
)

func (s *Server) actionScope(r *http.Request) (actions.Scope, error) {
	if s == nil || s.Application == nil {
		return actions.Scope{}, errors.New("product application is not configured")
	}
	workScope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		return actions.Scope{}, err
	}
	return actions.Scope{WorkspaceID: workScope.WorkspaceID, UserID: workScope.UserID}, nil
}

func (s *Server) v2CreateActionChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.Actions == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("action service is not configured"))
		return
	}
	var input createActionChallengeRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, err := actionIdempotencyKey(input.IdempotencyKey, r.Header.Get("Idempotency-Key"), false)
	if err != nil {
		writeActionError(w, err)
		return
	}
	scope, err := s.actionScope(r)
	if err != nil {
		writeProductError(w, err)
		return
	}
	challenge, err := s.Actions.CreateChallenge(r.Context(), scope, actions.CreateChallengeRequest{Target: input.target(), IdempotencyKey: key})
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, actionChallengeResponse{Action: challenge.Binding, Target: newActionTargetResponse(challenge.Target), ExpiresAt: challenge.ExpiresAt})
}

func (s *Server) v2ConfirmAction(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.Actions == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("action service is not configured"))
		return
	}
	var input confirmActionRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, err := actionIdempotencyKey(input.IdempotencyKey, r.Header.Get("Idempotency-Key"), true)
	if err != nil {
		writeActionError(w, err)
		return
	}
	scope, err := s.actionScope(r)
	if err != nil {
		writeProductError(w, err)
		return
	}
	confirmation, err := s.Actions.Confirm(r.Context(), scope, strings.TrimSpace(input.ChallengeID), input.target(), key)
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, actionConfirmationResponse{Action: confirmation.Binding, Receipt: confirmation.Receipt, Replayed: confirmation.Replayed})
}

// confirmConfiguredWorkMutation keeps the legacy REST route compatible while
// making the configured production server use the same challenge, version,
// replay and receipt boundary as the canonical action endpoint and MCP.
// Tests that construct a server without Actions retain the low-level route as
// a compatibility seam; the production composition always provides Actions.
func (s *Server) confirmConfiguredWorkMutation(w http.ResponseWriter, r *http.Request, scope work.Scope, entityType work.EntityType, entityID string, input stateMutationRequest, key string, operation connectors.SafeWriteOperation) {
	if s == nil || s.Actions == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("action service is not configured"))
		return
	}
	if strings.TrimSpace(input.ChallengeID) == "" {
		writeActionError(w, actions.ErrInvalidRequest)
		return
	}
	target := connectors.SafeWriteTarget{
		Operation:       operation,
		EntityType:      string(entityType),
		EntityID:        strings.TrimSpace(entityID),
		ExpectedVersion: input.ExpectedVersion,
	}
	confirmation, err := s.Actions.Confirm(r.Context(), actions.Scope{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID}, input.ChallengeID, target, key)
	if err != nil {
		writeActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, actionConfirmationResponse{Action: confirmation.Binding, Receipt: confirmation.Receipt, Replayed: confirmation.Replayed})
}

func (s *Server) confirmConfiguredWorkTrash(w http.ResponseWriter, r *http.Request, scope work.Scope, entityType work.EntityType, entityID string, input stateMutationRequest, key string) {
	s.confirmConfiguredWorkMutation(w, r, scope, entityType, entityID, input, key, connectors.SafeWriteOperationEntityTrash)
}

func (s *Server) confirmConfiguredWorkPurge(w http.ResponseWriter, r *http.Request, scope work.Scope, entityType work.EntityType, entityID string, input stateMutationRequest, key string) {
	s.confirmConfiguredWorkMutation(w, r, scope, entityType, entityID, input, key, connectors.SafeWriteOperationEntityPurge)
}

func (s *Server) v2CreateMCPToken(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.MCPTokenStore == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("MCP token service is not configured"))
		return
	}
	var input mcpTokenIssueRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	if len(input.Scopes) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("at least one MCP scope is required"))
		return
	}
	scope, err := s.actionScope(r)
	if err != nil {
		writeProductError(w, err)
		return
	}
	issued, err := s.MCPTokenStore.IssueForIdentity(input.Scopes, mcp.TokenIdentity{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID})
	if err != nil {
		writeMCPTokenError(w, err)
		return
	}
	token, err := issued.Reveal()
	if err != nil {
		writeError(w, http.StatusInternalServerError, errors.New("MCP token could not be revealed"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, mcpTokenResponse{ID: issued.ID, Token: token, Scopes: issued.Scopes, ExpiresAt: issued.ExpiresAt})
}

func (s *Server) v2RevokeMCPToken(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.MCPTokenStore == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("MCP token service is not configured"))
		return
	}
	scope, err := s.actionScope(r)
	if err != nil {
		writeProductError(w, err)
		return
	}
	err = s.MCPTokenStore.RevokeByIDForIdentity(r.PathValue("tokenID"), mcp.TokenIdentity{WorkspaceID: scope.WorkspaceID, UserID: scope.UserID})
	if err != nil {
		writeMCPTokenError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func actionIdempotencyKey(bodyKey, headerKey string, required bool) (string, error) {
	rawBodyKey := bodyKey
	rawHeaderKey := headerKey
	bodyKey = strings.TrimSpace(bodyKey)
	headerKey = strings.TrimSpace(headerKey)
	if rawBodyKey != "" && rawBodyKey != bodyKey {
		return "", errInvalidActionIdempotencyKey
	}
	if rawHeaderKey != "" && rawHeaderKey != headerKey {
		return "", errInvalidActionIdempotencyKey
	}
	if bodyKey != "" && headerKey != "" && bodyKey != headerKey {
		return "", errInvalidActionIdempotencyKey
	}
	key := bodyKey
	if key == "" {
		key = headerKey
	}
	if required && key == "" {
		return "", errActionIdempotencyRequired
	}
	if len(key) > 256 || strings.IndexFunc(key, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", errInvalidActionIdempotencyKey
	}
	return key, nil
}

func writeActionError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "action service failed"
	switch {
	case errors.Is(err, actions.ErrInvalidScope), errors.Is(err, actions.ErrInvalidRequest), errors.Is(err, actions.ErrUnsupportedAction),
		errors.Is(err, connectors.ErrInvalidWriteTarget), errors.Is(err, connectors.ErrInvalidRepository), errors.Is(err, connectors.ErrUnsupportedWrite),
		errors.Is(err, errInvalidActionIdempotencyKey), errors.Is(err, errActionIdempotencyRequired):
		status, message = http.StatusBadRequest, "invalid action request"
	case errors.Is(err, actions.ErrActionUnavailable):
		status, message = http.StatusServiceUnavailable, "action service is not configured"
	case errors.Is(err, actions.ErrChallengeNotFound), errors.Is(err, connectors.ErrInvalidChallenge):
		status, message = http.StatusNotFound, "action challenge was not found"
	case errors.Is(err, actions.ErrChallengeMismatch), errors.Is(err, actions.ErrInvalidActionState), errors.Is(err, connectors.ErrChallengeExpired),
		errors.Is(err, connectors.ErrChallengeUsed), errors.Is(err, connectors.ErrIdempotencyConflict), errors.Is(err, work.ErrIdempotencyConflict):
		status, message = http.StatusConflict, "action challenge is not valid for this request"
	case errors.Is(err, work.ErrVersionConflict), errors.Is(err, work.ErrDependenciesExist):
		status, message = http.StatusConflict, "action target changed or has dependent records"
	case errors.Is(err, work.ErrNotFound):
		status, message = http.StatusNotFound, "action target was not found"
	case errors.Is(err, connectors.ErrReceiptPersistence):
		status, message = http.StatusServiceUnavailable, "action receipt could not be persisted"
	case errors.Is(err, connectors.ErrReceiptUncertain):
		status, message = http.StatusConflict, "action outcome is uncertain; retry with the same idempotency key"
	case errors.Is(err, connectors.ErrInvalidProviderPayload):
		status, message = http.StatusBadGateway, "provider returned an invalid result"
	case errors.Is(err, connectors.ErrMissingUserID), errors.Is(err, connectors.ErrMissingChallenge):
		status, message = http.StatusBadRequest, "action confirmation is incomplete"
	}
	writeError(w, status, errors.New(message))
}

func writeMCPTokenError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "MCP token operation failed"
	switch {
	case errors.Is(err, mcp.ErrUnknownScope), errors.Is(err, mcp.ErrInvalidTokenIdentity), errors.Is(err, mcp.ErrInvalidToken):
		status, message = http.StatusBadRequest, "invalid MCP token request"
	case errors.Is(err, mcp.ErrTokenPersistence):
		status, message = http.StatusServiceUnavailable, "MCP token store is unavailable"
	}
	writeError(w, status, errors.New(message))
}
