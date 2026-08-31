package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/application"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/assistant"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/connectors"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/knowledge"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learningoverlay"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/platform"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/work"
)

func (s *Server) registerV2(mux *http.ServeMux) {
	if s.moduleEnabled(platform.ModulePlatform) {
		mux.HandleFunc("GET /api/v2/bootstrap", s.v2Bootstrap)
	}
	if s.moduleEnabled(platform.ModuleWork) {
		mux.HandleFunc("GET /api/v2/projects", s.v2Projects)
		mux.HandleFunc("POST /api/v2/projects", s.v2CreateProject)
		mux.HandleFunc("GET /api/v2/projects/{projectID}", s.v2GetProject)
		mux.HandleFunc("PATCH /api/v2/projects/{projectID}", s.v2UpdateProject)
		mux.HandleFunc("POST /api/v2/projects/{projectID}/restore", s.v2RestoreProject)
		mux.HandleFunc("GET /api/v2/projects/{projectID}/history", s.v2ProjectHistory)
		mux.HandleFunc("GET /api/v2/tasks", s.v2Tasks)
		mux.HandleFunc("POST /api/v2/projects/{projectID}/tasks", s.v2CreateProjectTask)
		mux.HandleFunc("GET /api/v2/tasks/{taskID}", s.v2GetTask)
		mux.HandleFunc("PATCH /api/v2/tasks/{taskID}", s.v2UpdateTask)
		mux.HandleFunc("POST /api/v2/tasks/{taskID}/restore", s.v2RestoreTask)
		mux.HandleFunc("GET /api/v2/tasks/{taskID}/history", s.v2TaskHistory)
		mux.HandleFunc("GET /api/v2/decisions", s.v2Decisions)
		mux.HandleFunc("POST /api/v2/decisions", s.v2CreateDecision)
		mux.HandleFunc("GET /api/v2/decisions/{decisionID}", s.v2GetDecision)
		mux.HandleFunc("PATCH /api/v2/decisions/{decisionID}", s.v2UpdateDecision)
		mux.HandleFunc("POST /api/v2/decisions/{decisionID}/restore", s.v2RestoreDecision)
		mux.HandleFunc("GET /api/v2/decisions/{decisionID}/history", s.v2DecisionHistory)
		if s.moduleEnabled(platform.ModuleActions) {
			// Delete-like operations stay unavailable when Actions is disabled;
			// this prevents a manifest from creating a direct-write escape hatch
			// around challenges and receipts.
			mux.HandleFunc("POST /api/v2/projects/{projectID}/trash", s.v2TrashProject)
			mux.HandleFunc("POST /api/v2/projects/{projectID}/purge", s.v2PurgeProject)
			mux.HandleFunc("POST /api/v2/tasks/{taskID}/trash", s.v2TrashTask)
			mux.HandleFunc("POST /api/v2/tasks/{taskID}/purge", s.v2PurgeTask)
			mux.HandleFunc("POST /api/v2/decisions/{decisionID}/trash", s.v2TrashDecision)
			mux.HandleFunc("POST /api/v2/decisions/{decisionID}/purge", s.v2PurgeDecision)
		}
	}
	if s.moduleEnabled(platform.ModuleKnowledge) {
		mux.HandleFunc("POST /api/v2/knowledge/sources", s.v2ImportSource)
		mux.HandleFunc("GET /api/v2/knowledge/sources", s.v2KnowledgeSources)
		mux.HandleFunc("GET /api/v2/knowledge/sources/{sourceID}", s.v2KnowledgeSource)
		mux.HandleFunc("GET /api/v2/knowledge/sources/{sourceID}/detail", s.v2KnowledgeSourceDetail)
		mux.HandleFunc("GET /api/v2/knowledge/items/{itemID}", s.v2KnowledgeItem)
		mux.HandleFunc("GET /api/v2/knowledge/revisions/{revisionID}", s.v2KnowledgeRevision)
		mux.HandleFunc("GET /api/v2/knowledge/chunks/{chunkID}", s.v2KnowledgeChunk)
		mux.HandleFunc("GET /api/v2/knowledge/claims/{claimID}", s.v2KnowledgeClaim)
		mux.HandleFunc("GET /api/v2/knowledge/search", s.v2KnowledgeSearch)
	}
	if s.moduleEnabled(platform.ModuleConnectors) {
		mux.HandleFunc("POST /api/v2/connectors/drive/sync", s.v2DriveSync)
		mux.HandleFunc("POST /api/v2/connectors/github/sync", s.v2GitHubSync)
	}
	if s.moduleEnabled(platform.ModuleLearning) {
		mux.HandleFunc("POST /api/v2/learning/observations", s.v2RecordLearningObservation)
		mux.HandleFunc("GET /api/v2/learning/observations", s.v2ListLearningObservations)
	}
	if s.moduleEnabled(platform.ModuleAssistant) {
		mux.HandleFunc("POST /api/v2/assistant/conversations", s.v2StartConversation)
		mux.HandleFunc("GET /api/v2/assistant/conversations", s.v2ListConversations)
		mux.HandleFunc("GET /api/v2/assistant/conversations/{conversationID}", s.v2GetConversation)
		mux.HandleFunc("POST /api/v2/assistant/conversations/{conversationID}/messages", s.v2SendMessage)
	}
	if s.moduleEnabled(platform.ModuleActions) {
		mux.HandleFunc("POST /api/v2/actions/challenges", s.v2CreateActionChallenge)
		mux.HandleFunc("POST /api/v2/actions/confirm", s.v2ConfirmAction)
	}
	if s.moduleEnabled(platform.ModuleMCP) {
		mux.HandleFunc("POST /api/v2/mcp/tokens", s.v2CreateMCPToken)
		mux.HandleFunc("DELETE /api/v2/mcp/tokens/{tokenID}", s.v2RevokeMCPToken)
	}
}

func (s *Server) v2GetProject(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Work.GetProject(r.Context(), scope, r.PathValue("projectID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) v2Ready(w http.ResponseWriter) bool {
	if s.Application == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("product application is not configured"))
		return false
	}
	return true
}

func (s *Server) v2Bootstrap(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	result, err := s.Application.BootstrapWithModules(r.Context(), parseBoolQuery(r.URL.Query().Get("includeTrashed")), application.BootstrapModules{
		IncludeWork:      s.moduleEnabled(platform.ModuleWork),
		IncludeKnowledge: s.moduleEnabled(platform.ModuleKnowledge),
		IncludeAssistant: s.moduleEnabled(platform.ModuleAssistant),
		Capabilities:     s.bootstrapCapabilities(),
	})
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) bootstrapCapabilities() map[string]bool {
	return map[string]bool{
		platform.ModulePlatform:   s.moduleEnabled(platform.ModulePlatform),
		platform.ModuleWork:       s.moduleEnabled(platform.ModuleWork),
		platform.ModuleKnowledge:  s.moduleEnabled(platform.ModuleKnowledge),
		platform.ModuleConnectors: s.moduleEnabled(platform.ModuleConnectors),
		platform.ModuleAssistant:  s.moduleEnabled(platform.ModuleAssistant),
		platform.ModuleActions:    s.moduleEnabled(platform.ModuleActions),
		platform.ModuleMCP:        s.moduleEnabled(platform.ModuleMCP),
		platform.ModuleLearning:   s.moduleEnabled(platform.ModuleLearning),
	}
}

func (s *Server) v2Projects(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	items, err := s.Application.Work.ListProjects(r.Context(), scope, work.ListOptions{Limit: parseLimit(r.URL.Query().Get("limit")), IncludeTrashed: parseBoolQuery(r.URL.Query().Get("includeTrashed"))})
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": items})
}

func (s *Server) v2CreateProject(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input work.CreateProjectInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, ok := mutationKey(w, r)
	if !ok {
		return
	}
	project, err := s.Application.CreateProject(r.Context(), input, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) v2UpdateProject(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input struct {
		Name            *string             `json:"name"`
		Description     *string             `json:"description"`
		Status          *work.ProjectStatus `json:"status"`
		ExpectedVersion int64               `json:"expectedVersion"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, ok := mutationKey(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	project, err := s.Application.Work.UpdateProject(r.Context(), scope, r.PathValue("projectID"), work.ProjectPatch{Name: input.Name, Description: input.Description, Status: input.Status}, input.ExpectedVersion, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) v2TrashProject(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	if s.Actions != nil {
		s.confirmConfiguredWorkTrash(w, r, scope, work.EntityProject, r.PathValue("projectID"), input, key)
		return
	}
	project, err := s.Application.Work.TrashProject(r.Context(), scope, r.PathValue("projectID"), input.ExpectedVersion, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) v2RestoreProject(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	project, err := s.Application.Work.RestoreProject(r.Context(), scope, r.PathValue("projectID"), input.ExpectedVersion, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) v2PurgeProject(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	if s.Actions != nil {
		s.confirmConfiguredWorkPurge(w, r, scope, work.EntityProject, r.PathValue("projectID"), input, key)
		return
	}
	writeError(w, http.StatusServiceUnavailable, errors.New("action service is not configured"))
}

func (s *Server) v2ProjectHistory(w http.ResponseWriter, r *http.Request) {
	s.v2WorkHistory(w, r, work.EntityProject, r.PathValue("projectID"))
}

func (s *Server) v2Tasks(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	items, err := s.Application.Work.ListTasks(r.Context(), scope, strings.TrimSpace(r.URL.Query().Get("projectId")), work.ListOptions{Limit: parseLimit(r.URL.Query().Get("limit")), IncludeTrashed: parseBoolQuery(r.URL.Query().Get("includeTrashed"))})
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": items})
}

func (s *Server) v2GetTask(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Work.GetTask(r.Context(), scope, r.PathValue("taskID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) v2CreateProjectTask(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input work.CreateTaskInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	input.ProjectID = r.PathValue("projectID")
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("Idempotency-Key header is required"))
		return
	}
	task, err := s.Application.CreateTask(r.Context(), input, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) v2UpdateTask(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input struct {
		ProjectID       *string            `json:"projectId"`
		Title           *string            `json:"title"`
		Description     *string            `json:"description"`
		Status          *work.TaskStatus   `json:"status"`
		Priority        *work.TaskPriority `json:"priority"`
		DueAt           **time.Time        `json:"dueAt"`
		ExpectedVersion int64              `json:"expectedVersion"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, ok := mutationKey(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	task, err := s.Application.Work.UpdateTask(r.Context(), scope, r.PathValue("taskID"), work.TaskPatch{ProjectID: input.ProjectID, Title: input.Title, Description: input.Description, Status: input.Status, Priority: input.Priority, DueAt: input.DueAt}, input.ExpectedVersion, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) v2TrashTask(w http.ResponseWriter, r *http.Request) {
	s.v2TaskState(w, r, true)
}

func (s *Server) v2RestoreTask(w http.ResponseWriter, r *http.Request) {
	s.v2TaskState(w, r, false)
}

func (s *Server) v2TaskState(w http.ResponseWriter, r *http.Request, trash bool) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	var task work.Task
	if trash {
		if s.Actions != nil {
			s.confirmConfiguredWorkTrash(w, r, scope, work.EntityTask, r.PathValue("taskID"), input, key)
			return
		}
		task, err = s.Application.Work.TrashTask(r.Context(), scope, r.PathValue("taskID"), input.ExpectedVersion, key)
	} else {
		task, err = s.Application.Work.RestoreTask(r.Context(), scope, r.PathValue("taskID"), input.ExpectedVersion, key)
	}
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) v2PurgeTask(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	if s.Actions != nil {
		s.confirmConfiguredWorkPurge(w, r, scope, work.EntityTask, r.PathValue("taskID"), input, key)
		return
	}
	writeError(w, http.StatusServiceUnavailable, errors.New("action service is not configured"))
}

func (s *Server) v2TaskHistory(w http.ResponseWriter, r *http.Request) {
	s.v2WorkHistory(w, r, work.EntityTask, r.PathValue("taskID"))
}

func (s *Server) v2Decisions(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	items, err := s.Application.Work.ListDecisions(r.Context(), scope, strings.TrimSpace(r.URL.Query().Get("projectId")), work.ListOptions{Limit: parseLimit(r.URL.Query().Get("limit")), IncludeTrashed: parseBoolQuery(r.URL.Query().Get("includeTrashed"))})
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"decisions": items})
}

func (s *Server) v2GetDecision(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Work.GetDecision(r.Context(), scope, r.PathValue("decisionID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) v2CreateDecision(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input work.CreateDecisionInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, ok := mutationKey(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	decision, err := s.Application.Work.CreateDecision(r.Context(), scope, input, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, decision)
}

func (s *Server) v2UpdateDecision(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input struct {
		ProjectID       *string              `json:"projectId"`
		Title           *string              `json:"title"`
		Context         *string              `json:"context"`
		Outcome         *string              `json:"outcome"`
		Rationale       *string              `json:"rationale"`
		Status          *work.DecisionStatus `json:"status"`
		ExpectedVersion int64                `json:"expectedVersion"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, ok := mutationKey(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	decision, err := s.Application.Work.UpdateDecision(r.Context(), scope, r.PathValue("decisionID"), work.DecisionPatch{ProjectID: input.ProjectID, Title: input.Title, Context: input.Context, Outcome: input.Outcome, Rationale: input.Rationale, Status: input.Status}, input.ExpectedVersion, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) v2DecisionState(w http.ResponseWriter, r *http.Request, trash bool) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	var decision work.Decision
	if trash {
		if s.Actions != nil {
			s.confirmConfiguredWorkTrash(w, r, scope, work.EntityDecision, r.PathValue("decisionID"), input, key)
			return
		}
		decision, err = s.Application.Work.TrashDecision(r.Context(), scope, r.PathValue("decisionID"), input.ExpectedVersion, key)
	} else {
		decision, err = s.Application.Work.RestoreDecision(r.Context(), scope, r.PathValue("decisionID"), input.ExpectedVersion, key)
	}
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) v2TrashDecision(w http.ResponseWriter, r *http.Request) {
	s.v2DecisionState(w, r, true)
}

func (s *Server) v2RestoreDecision(w http.ResponseWriter, r *http.Request) {
	s.v2DecisionState(w, r, false)
}

func (s *Server) v2PurgeDecision(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	input, key, ok := stateMutationInput(w, r)
	if !ok {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	if s.Actions != nil {
		s.confirmConfiguredWorkPurge(w, r, scope, work.EntityDecision, r.PathValue("decisionID"), input, key)
		return
	}
	writeError(w, http.StatusServiceUnavailable, errors.New("action service is not configured"))
}

func (s *Server) v2DecisionHistory(w http.ResponseWriter, r *http.Request) {
	s.v2WorkHistory(w, r, work.EntityDecision, r.PathValue("decisionID"))
}

func (s *Server) v2WorkHistory(w http.ResponseWriter, r *http.Request, entityType work.EntityType, id string) {
	if !s.v2Ready(w) {
		return
	}
	scope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	items, err := s.Application.Work.History(r.Context(), scope, entityType, id)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": items})
}

type stateMutationRequest struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	ChallengeID     string `json:"challengeId"`
}

func stateMutationInput(w http.ResponseWriter, r *http.Request) (stateMutationRequest, string, bool) {
	var input stateMutationRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return stateMutationRequest{}, "", false
	}
	key, ok := mutationKey(w, r)
	return input, key, ok
}

func mutationKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("Idempotency-Key header is required"))
		return "", false
	}
	return key, true
}

func (s *Server) v2ImportSource(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input application.ManualSourceInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	key, ok := mutationKey(w, r)
	if !ok {
		return
	}
	result, err := s.Application.ImportManualSource(r.Context(), input, key)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) v2KnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, errors.New("q query parameter is required"))
		return
	}
	result, err := s.Application.SearchKnowledge(r.Context(), query, parseLimit(r.URL.Query().Get("limit")))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": result})
}

type driveSyncRequest struct {
	Cursor   string `json:"cursor"`
	PageSize int    `json:"pageSize"`
}

func (s *Server) v2DriveSync(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.DriveSync == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("Google Drive sync is not configured"))
		return
	}
	var input driveSyncRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	_, _, workspace, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	syncService, err := s.DriveSync(workspace.ID)
	if err != nil {
		writeConnectorError(w, err)
		return
	}
	result, err := syncService.Sync(r.Context(), connectors.DriveSyncRequest{
		WorkspaceID: workspace.ID,
		Cursor:      connectors.DriveCursor{Token: strings.TrimSpace(input.Cursor)},
		PageSize:    input.PageSize,
	})
	if err != nil {
		writeConnectorError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type githubSyncRequest struct {
	Repository string `json:"repository"`
	Cursor     string `json:"cursor"`
	PageSize   int    `json:"pageSize"`
}

func (s *Server) v2GitHubSync(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.GitHubSync == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("GitHub sync is not configured"))
		return
	}
	var input githubSyncRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	_, _, workspace, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	syncService, err := s.GitHubSync(workspace.ID)
	if err != nil {
		writeConnectorError(w, err)
		return
	}
	result, err := syncService.SyncIssues(r.Context(), connectors.GitHubImportRequest{
		WorkspaceID: workspace.ID,
		Repository:  strings.TrimSpace(input.Repository),
		Cursor:      strings.TrimSpace(input.Cursor),
		PageSize:    input.PageSize,
	})
	if err != nil {
		writeConnectorError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) v2RecordLearningObservation(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.LearningOverlay == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("learning overlay is not configured"))
		return
	}
	var input learningoverlay.ObservationInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	workScope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.LearningOverlay.Record(r.Context(), learningoverlay.Scope{
		WorkspaceID: workScope.WorkspaceID,
		UserID:      workScope.UserID,
	}, input)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) v2ListLearningObservations(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	if s.LearningOverlay == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("learning overlay is not configured"))
		return
	}
	workScope, _, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	items, err := s.LearningOverlay.List(r.Context(), learningoverlay.Scope{
		WorkspaceID: workScope.WorkspaceID,
		UserID:      workScope.UserID,
	}, parseLimit(r.URL.Query().Get("limit")))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observations": items})
}

type knowledgeSourceResponse struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	URI         string         `json:"uri"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Version     int64          `json:"version"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   *time.Time     `json:"deletedAt,omitempty"`
}

type knowledgeSourceItemResponse struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspaceId"`
	SourceID          string     `json:"sourceId"`
	ExternalID        string     `json:"externalId"`
	Title             string     `json:"title"`
	URI               string     `json:"uri"`
	MIMEType          string     `json:"mimeType"`
	CurrentRevisionID string     `json:"currentRevisionId,omitempty"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty"`
}

type knowledgeRevisionResponse struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspaceId"`
	SourceItemID string    `json:"sourceItemId"`
	RevisionKey  string    `json:"revisionKey"`
	ContentHash  string    `json:"contentHash"`
	ContentType  string    `json:"contentType"`
	SourceURI    string    `json:"sourceUri"`
	Content      string    `json:"content"`
	ModifiedAt   time.Time `json:"modifiedAt,omitempty"`
	IngestedAt   time.Time `json:"ingestedAt"`
}

type knowledgeEvidenceResponse struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspaceId"`
	ClaimID          string    `json:"claimId"`
	SourceRevisionID string    `json:"sourceRevisionId"`
	ChunkID          string    `json:"chunkId,omitempty"`
	Locator          string    `json:"locator"`
	Quote            string    `json:"quote"`
	Freshness        string    `json:"freshness"`
	CreatedAt        time.Time `json:"createdAt"`
}

type knowledgeClaimResponse struct {
	v2KnowledgeClaimPayload
	Evidence []knowledgeEvidenceResponse `json:"evidence"`
}

type v2KnowledgeClaimPayload struct {
	ID          string                   `json:"id"`
	WorkspaceID string                   `json:"workspaceId"`
	TopicID     string                   `json:"topicId,omitempty"`
	Statement   string                   `json:"statement"`
	Certainty   knowledge.ClaimCertainty `json:"certainty"`
	Freshness   knowledge.ClaimFreshness `json:"freshness"`
	Version     int64                    `json:"version"`
	CreatedAt   time.Time                `json:"createdAt"`
	UpdatedAt   time.Time                `json:"updatedAt"`
	DeletedAt   *time.Time               `json:"deletedAt,omitempty"`
}

type v2KnowledgeChunkPayload struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	RevisionID  string    `json:"revisionId"`
	Ordinal     int       `json:"ordinal"`
	Text        string    `json:"text"`
	TokenCount  int       `json:"tokenCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toKnowledgeSourceResponse(source knowledge.KnowledgeSource) knowledgeSourceResponse {
	return knowledgeSourceResponse{ID: source.ID, WorkspaceID: source.WorkspaceID, Kind: source.Kind, Name: source.Name, URI: source.URI, Metadata: source.Metadata, Version: source.Version, CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt, DeletedAt: source.DeletedAt}
}

func toKnowledgeSourceItemResponse(item knowledge.SourceItem) knowledgeSourceItemResponse {
	return knowledgeSourceItemResponse{ID: item.ID, WorkspaceID: item.WorkspaceID, SourceID: item.SourceID, ExternalID: item.ExternalID, Title: item.Title, URI: item.URI, MIMEType: item.MIMEType, CurrentRevisionID: item.CurrentRevisionID, Version: item.Version, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt}
}

func toKnowledgeRevisionResponse(revision knowledge.SourceRevision) knowledgeRevisionResponse {
	return knowledgeRevisionResponse{ID: revision.ID, WorkspaceID: revision.WorkspaceID, SourceItemID: revision.SourceItemID, RevisionKey: revision.RevisionKey, ContentHash: revision.ContentHash, ContentType: revision.ContentType, SourceURI: revision.SourceURI, Content: revision.Content, ModifiedAt: revision.ModifiedAt, IngestedAt: revision.IngestedAt}
}

func toKnowledgeEvidenceResponse(item knowledge.ClaimEvidence) knowledgeEvidenceResponse {
	return knowledgeEvidenceResponse{ID: item.ID, WorkspaceID: item.WorkspaceID, ClaimID: item.ClaimID, SourceRevisionID: item.SourceRevisionID, ChunkID: item.ChunkID, Locator: item.Locator, Quote: item.Quote, Freshness: string(item.Freshness), CreatedAt: item.CreatedAt}
}

func toV2KnowledgeClaimPayload(claim knowledge.KnowledgeClaim) v2KnowledgeClaimPayload {
	return v2KnowledgeClaimPayload{ID: claim.ID, WorkspaceID: claim.WorkspaceID, TopicID: claim.TopicID, Statement: claim.Statement, Certainty: claim.Certainty, Freshness: claim.Freshness, Version: claim.Version, CreatedAt: claim.CreatedAt, UpdatedAt: claim.UpdatedAt, DeletedAt: claim.DeletedAt}
}

func toV2KnowledgeChunkPayload(chunk knowledge.KnowledgeChunk) v2KnowledgeChunkPayload {
	return v2KnowledgeChunkPayload{ID: chunk.ID, WorkspaceID: chunk.WorkspaceID, RevisionID: chunk.RevisionID, Ordinal: chunk.Ordinal, Text: chunk.Text, TokenCount: chunk.TokenCount, CreatedAt: chunk.CreatedAt}
}

func (s *Server) v2KnowledgeSources(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	_, scope, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	items, err := s.Application.Knowledge.ListSources(r.Context(), scope, parseLimit(r.URL.Query().Get("limit")))
	if err != nil {
		writeProductError(w, err)
		return
	}
	result := make([]knowledgeSourceResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toKnowledgeSourceResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": result})
}

func (s *Server) v2KnowledgeSource(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	_, scope, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Knowledge.GetSource(r.Context(), scope, r.PathValue("sourceID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toKnowledgeSourceResponse(item))
}

type knowledgeSourceDetailResponse struct {
	Source    knowledgeSourceResponse       `json:"source"`
	Items     []knowledgeSourceItemResponse `json:"items"`
	Revisions []knowledgeRevisionResponse   `json:"revisions"`
	Chunks    []v2KnowledgeChunkPayload     `json:"chunks"`
	Evidence  []knowledgeEvidenceResponse   `json:"evidence"`
}

func toKnowledgeSourceDetailResponse(detail knowledge.SourceDetail) knowledgeSourceDetailResponse {
	result := knowledgeSourceDetailResponse{
		Source:    toKnowledgeSourceResponse(detail.Source),
		Items:     make([]knowledgeSourceItemResponse, 0, len(detail.Items)),
		Revisions: make([]knowledgeRevisionResponse, 0, len(detail.Revisions)),
		Chunks:    make([]v2KnowledgeChunkPayload, 0, len(detail.Chunks)),
		Evidence:  make([]knowledgeEvidenceResponse, 0, len(detail.Evidence)),
	}
	for _, item := range detail.Items {
		result.Items = append(result.Items, toKnowledgeSourceItemResponse(item))
	}
	for _, item := range detail.Revisions {
		result.Revisions = append(result.Revisions, toKnowledgeRevisionResponse(item))
	}
	for _, item := range detail.Chunks {
		result.Chunks = append(result.Chunks, toV2KnowledgeChunkPayload(item))
	}
	for _, item := range detail.Evidence {
		result.Evidence = append(result.Evidence, toKnowledgeEvidenceResponse(item))
	}
	return result
}

func (s *Server) v2KnowledgeSourceDetail(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	detail, err := s.Application.GetKnowledgeSourceDetail(r.Context(), r.PathValue("sourceID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toKnowledgeSourceDetailResponse(detail))
}

func (s *Server) v2KnowledgeItem(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	_, scope, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Knowledge.GetSourceItem(r.Context(), scope, r.PathValue("itemID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toKnowledgeSourceItemResponse(item))
}

func (s *Server) v2KnowledgeRevision(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	_, scope, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Knowledge.GetRevision(r.Context(), scope, r.PathValue("revisionID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toKnowledgeRevisionResponse(item))
}

func (s *Server) v2KnowledgeChunk(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	_, scope, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	item, err := s.Application.Knowledge.GetChunk(r.Context(), scope, r.PathValue("chunkID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toV2KnowledgeChunkPayload(item))
}

func (s *Server) v2KnowledgeClaim(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	_, scope, _, err := s.Application.Scope(r.Context())
	if err != nil {
		writeProductError(w, err)
		return
	}
	claim, err := s.Application.Knowledge.GetClaim(r.Context(), scope, r.PathValue("claimID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	evidence, err := s.Application.Knowledge.ListClaimEvidence(r.Context(), scope, claim.ID)
	if err != nil {
		writeProductError(w, err)
		return
	}
	result := knowledgeClaimResponse{v2KnowledgeClaimPayload: toV2KnowledgeClaimPayload(claim), Evidence: make([]knowledgeEvidenceResponse, 0, len(evidence))}
	for _, item := range evidence {
		result.Evidence = append(result.Evidence, toKnowledgeEvidenceResponse(item))
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) v2StartConversation(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input v2AssistantMessageRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	contextRef, err := input.contextRef()
	if err != nil {
		writeProductError(w, err)
		return
	}
	result, err := s.Application.AskWithContext(r.Context(), "", input.Message, contextRef)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) v2ListConversations(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	items, err := s.Application.ListConversations(r.Context(), parseLimit(r.URL.Query().Get("limit")))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

func (s *Server) v2GetConversation(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	item, err := s.Application.GetConversation(r.Context(), r.PathValue("conversationID"))
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) v2SendMessage(w http.ResponseWriter, r *http.Request) {
	if !s.v2Ready(w) {
		return
	}
	var input v2AssistantMessageRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	contextRef, err := input.contextRef()
	if err != nil {
		writeProductError(w, err)
		return
	}
	result, err := s.Application.AskWithContext(r.Context(), r.PathValue("conversationID"), input.Message, contextRef)
	if err != nil {
		writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type v2AssistantMessageRequest struct {
	Message     string `json:"message"`
	ContextType string `json:"contextType,omitempty"`
	ContextID   string `json:"contextId,omitempty"`
}

func (input v2AssistantMessageRequest) contextRef() (assistant.ContextRef, error) {
	return (assistant.ContextRef{Type: input.ContextType, ID: input.ContextID}).Normalize()
}

func parseLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func parseBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func writeProductError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, work.ErrAlreadyExists), errors.Is(err, work.ErrVersionConflict), errors.Is(err, work.ErrIdempotencyConflict), errors.Is(err, knowledge.ErrConflict), errors.Is(err, application.ErrConversationContextConflict):
		status = http.StatusConflict
	case errors.Is(err, work.ErrAlreadyTrashed), errors.Is(err, work.ErrNotTrashed), errors.Is(err, work.ErrRecordTrashed), errors.Is(err, work.ErrDependenciesExist):
		status = http.StatusConflict
	case errors.Is(err, work.ErrPurgeNotReady):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, work.ErrNotFound), errors.Is(err, knowledge.ErrNotFound), errors.Is(err, application.ErrConversationNotFound):
		status = http.StatusNotFound
	case errors.As(err, new(*work.ValidationError)), errors.Is(err, knowledge.ErrInvalidInput), errors.Is(err, application.ErrInvalidInput), errors.Is(err, assistant.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, learningoverlay.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, application.ErrWorkspaceMappingRequired), errors.Is(err, application.ErrWorkspaceUnavailable):
		status = http.StatusServiceUnavailable
	case errors.Is(err, knowledge.ErrWorkspaceMismatch):
		status = http.StatusForbidden
	}
	writeError(w, status, err)
}

func writeConnectorError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, connectors.ErrInvalidCursor), errors.Is(err, connectors.ErrInvalidRevisionIdentity),
		errors.Is(err, connectors.ErrInvalidWorkspaceID), errors.Is(err, connectors.ErrInvalidRepository),
		errors.Is(err, connectors.ErrInvalidPageSize), errors.Is(err, connectors.ErrInvalidSyncState),
		errors.Is(err, connectors.ErrInvalidProviderPayload):
		status = http.StatusBadRequest
	case errors.Is(err, connectors.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, connectors.ErrRevisionAlreadyExists), errors.Is(err, connectors.ErrWriteConflict):
		status = http.StatusConflict
	}
	writeError(w, status, err)
}
