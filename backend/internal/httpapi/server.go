package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/ai"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/auth"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/learning"
	"github.com/DattruongRyan1912/Dev-English/backend/internal/store"
)

type Server struct {
	Service        *learning.Service
	Logger         *slog.Logger
	Auth           *auth.Manager
	StrictAuth     bool
	AllowedOrigins []string
}

func NewServer(service *learning.Service, logger *slog.Logger, managers ...*auth.Manager) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	var manager *auth.Manager
	if len(managers) > 0 {
		manager = managers[0]
	}
	return &Server{Service: service, Logger: logger, Auth: manager}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("POST /api/v1/auth/session", s.authSession)
	mux.HandleFunc("GET /api/v1/auth/me", s.authMe)
	mux.HandleFunc("GET /api/v1/home", s.home)
	mux.HandleFunc("GET /api/v1/practice", s.practice)
	mux.HandleFunc("GET /api/v1/review/due", s.reviewDue)
	mux.HandleFunc("GET /api/v1/review", s.reviewLibrary)
	mux.HandleFunc("POST /api/v1/review/", s.reviewAction)
	mux.HandleFunc("GET /api/v1/progress", s.progress)
	mux.HandleFunc("GET /api/v1/diagnostic/questions", s.diagnosticQuestions)
	mux.HandleFunc("POST /api/v1/diagnostic", s.submitDiagnostic)
	mux.HandleFunc("GET /api/v1/diagnostic", s.diagnostic)
	mux.HandleFunc("GET /api/v1/roleplay/scenarios", s.roleplayScenarios)
	mux.HandleFunc("POST /api/v1/roleplay/conversations", s.startRoleplay)
	mux.HandleFunc("POST /api/v1/roleplay/conversations/", s.roleplayTurn)
	mux.HandleFunc("POST /api/v1/copilot", s.copilot)
	mux.HandleFunc("GET /api/v1/vocabulary", s.vocabulary)
	mux.HandleFunc("GET /api/v1/vocabulary/graph", s.vocabularyGraph)
	mux.HandleFunc("POST /api/v1/integrations/github/import", s.importGitHub)
	mux.HandleFunc("GET /api/v1/analytics", s.analytics)
	mux.HandleFunc("GET /api/v1/speaking/weekly", s.weeklySpeaking)
	mux.HandleFunc("GET /api/v1/usage", s.usage)
	mux.HandleFunc("GET /api/v1/privacy/export", s.exportData)
	mux.HandleFunc("DELETE /api/v1/privacy/data", s.deleteData)
	mux.HandleFunc("POST /api/v1/speaking/transcribe", s.transcribe)
	mux.HandleFunc("POST /api/v1/speaking/transcript", s.saveTranscript)
	mux.HandleFunc("POST /api/v1/speaking/assess", s.assessSpeaking)
	mux.HandleFunc("POST /api/v1/speaking/synthesize", s.synthesize)
	mux.HandleFunc("GET /api/v1/settings", s.settings)
	mux.HandleFunc("GET /api/v1/settings/test", s.testSettings)
	mux.HandleFunc("PUT /api/v1/settings", s.updateSettings)
	mux.HandleFunc("POST /api/v1/missions/daily", s.createDailyMission)
	mux.HandleFunc("POST /api/v1/work-context", s.workContext)
	mux.HandleFunc("POST /api/v1/missions/", s.missionAction)
	return withCORS(withRequestLog(s.withAuth(mux), s.Logger), s.AllowedOrigins)
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || (r.URL.Path == "/api/v1/auth/session" && r.Method == http.MethodPost) {
			next.ServeHTTP(w, r)
			return
		}
		if s.Auth == nil || !s.Auth.Enabled() {
			if s.StrictAuth {
				writeError(w, http.StatusServiceUnavailable, errors.New("authentication is not configured"))
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, errors.New("bearer token is required"))
			return
		}
		claims, err := s.Auth.Parse(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		ctx := store.WithUser(r.Context(), claims.Subject)
		if err := s.Service.Store.EnsureUser(ctx, domain.User{ID: claims.Subject, DisplayName: "Developer", CEFR: "A1", CreatedAt: time.Now().UTC()}); err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("authenticated user is not available"))
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID      string `json:"userId"`
		DisplayName string `json:"displayName"`
		CEFR        string `json:"cefr"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	input.UserID = strings.TrimSpace(input.UserID)
	if input.UserID == "" {
		input.UserID = "user-1"
	}
	if len(input.UserID) > 128 || strings.ContainsAny(input.UserID, "\r\n") {
		writeError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	if s.Auth != nil && s.Auth.Enabled() && !s.Auth.BootstrapAllowed(r.Header.Get("X-Bootstrap-Key")) {
		writeError(w, http.StatusForbidden, errors.New("bootstrap authorization failed"))
		return
	}
	user := domain.User{ID: input.UserID, DisplayName: strings.TrimSpace(input.DisplayName), CEFR: strings.TrimSpace(input.CEFR), CreatedAt: time.Now().UTC()}
	ctx := store.WithUser(r.Context(), user.ID)
	if err := s.Service.Store.EnsureUser(ctx, user); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	response := map[string]any{"user": user}
	if s.Auth != nil && s.Auth.Enabled() {
		token, claims, err := s.Auth.Issue(user.ID, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		response["token"] = token
		response["expiresAt"] = time.Unix(claims.ExpiresAt, 0).UTC()
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.Service.Store.User(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "devenglish-backend", "time": time.Now().UTC(), "provider": domain.ProviderStatus{Name: s.Service.AI.Name(), Mode: providerMode(s.Service.AI), Configured: s.Service.AI.Configured()}})
}

func providerMode(provider ai.Provider) string {
	if provider == nil || !provider.Configured() {
		if provider != nil && provider.Name() == "deterministic-fallback" {
			return "deterministic-fallback"
		}
		return "unavailable"
	}
	return "primary"
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Service.Home(r.Context()))
}
func (s *Server) practice(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"modes": s.Service.Practice(r.Context())})
}
func (s *Server) reviewDue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.Service.ReviewDue(r.Context())})
}

func (s *Server) reviewLibrary(w http.ResponseWriter, r *http.Request) {
	items, err := s.Service.ReviewLibrary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) reviewAction(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/review/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, errors.New("review endpoint not found"))
		return
	}
	var input struct {
		Success bool    `json:"success"`
		Score   float64 `json:"score"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	result, err := s.Service.SubmitReview(r.Context(), parts[0], parts[1], input.Success, input.Score)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) progress(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Service.Progress(r.Context()))
}

func (s *Server) diagnosticQuestions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"questions": s.Service.DiagnosticQuestions()})
}

func (s *Server) submitDiagnostic(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Responses []domain.DiagnosticResponse `json:"responses"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	result, err := s.Service.RunDiagnostic(r.Context(), input.Responses)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) diagnostic(w http.ResponseWriter, r *http.Request) {
	result, err := s.Service.Diagnostic(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) roleplayScenarios(w http.ResponseWriter, r *http.Request) {
	scenarios, err := s.Service.RoleplayScenarios(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scenarios": scenarios})
}

func (s *Server) startRoleplay(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ScenarioID string `json:"scenarioId"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	conversation, err := s.Service.StartRoleplay(r.Context(), strings.TrimSpace(input.ScenarioID))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusCreated, conversation)
}

func (s *Server) roleplayTurn(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/roleplay/conversations/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "turns" {
		writeError(w, http.StatusNotFound, errors.New("roleplay turn endpoint not found"))
		return
	}
	var input domain.RoleplayTurnRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	input.ConversationID = parts[0]
	result, err := s.Service.RoleplayTurn(r.Context(), input)
	if err != nil {
		status := featureErrorStatus(err, http.StatusBadRequest)
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) copilot(w http.ResponseWriter, r *http.Request) {
	var input domain.CopilotRequest
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	result, err := s.Service.Copilot(r.Context(), input)
	if err != nil {
		writeError(w, featureErrorStatus(err, http.StatusBadGateway), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) vocabulary(w http.ResponseWriter, r *http.Request) {
	items, err := s.Service.Store.AllVocabulary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) vocabularyGraph(w http.ResponseWriter, r *http.Request) {
	graph, err := s.Service.VocabularyGraph(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (s *Server) importGitHub(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	if strings.TrimSpace(input.URL) == "" {
		writeError(w, http.StatusBadRequest, errors.New("GitHub URL is required"))
		return
	}
	result, err := s.Service.ImportGitHub(r.Context(), input.URL)
	if err != nil {
		writeError(w, githubErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) analytics(w http.ResponseWriter, r *http.Request) {
	result, err := s.Service.Analytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) weeklySpeaking(w http.ResponseWriter, r *http.Request) {
	result, err := s.Service.WeeklySpeaking(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	result, err := s.Service.UsageSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) exportData(w http.ResponseWriter, r *http.Request) {
	result, err := s.Service.ExportData(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) deleteData(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || !s.Auth.Enabled() {
		writeError(w, http.StatusForbidden, errors.New("data deletion requires authenticated mode"))
		return
	}
	if err := s.Service.Store.DeleteUserData(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) transcribe(w http.ResponseWriter, r *http.Request) {
	audio, mimeType, err := readAudio(w, r)
	if err != nil {
		return
	}
	session, err := s.Service.TranscribeAudio(r.Context(), r.FormValue("missionId"), mimeType, audio)
	if err != nil {
		writeSpeechError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) saveTranscript(w http.ResponseWriter, r *http.Request) {
	var input struct {
		MissionID  string `json:"missionId"`
		Transcript string `json:"transcript"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	session, err := s.Service.SaveTranscript(r.Context(), input.MissionID, input.Transcript)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) assessSpeaking(w http.ResponseWriter, r *http.Request) {
	audio, mimeType, err := readAudio(w, r)
	if err != nil {
		return
	}
	sessionID := strings.TrimSpace(r.FormValue("sessionId"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, errors.New("sessionId is required"))
		return
	}
	session, err := s.Service.AssessSpeaking(r.Context(), sessionID, mimeType, r.FormValue("reference"), audio)
	if err != nil {
		writeSpeechError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) synthesize(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	audio, err := s.Service.Synthesize(r.Context(), input.Text, input.Voice)
	if err != nil {
		writeSpeechError(w, err)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

func readAudio(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("multipart audio body is required"))
		return nil, "", err
	}
	file, header, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("audio file is required"))
		return nil, "", err
	}
	defer file.Close()
	audio, err := io.ReadAll(io.LimitReader(file, 25<<20+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("could not read audio"))
		return nil, "", err
	}
	if len(audio) == 0 || len(audio) > 25<<20 {
		writeError(w, http.StatusRequestEntityTooLarge, errors.New("audio must be between 1 byte and 25 MB"))
		return nil, "", errors.New("invalid audio size")
	}
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "audio/webm"
	}
	return audio, mimeType, nil
}

func writeSpeechError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if errors.Is(err, ai.ErrProviderUnavailable) {
		status = http.StatusServiceUnavailable
	}
	status = featureErrorStatus(err, status)
	writeError(w, status, err)
}

func featureErrorStatus(err error, fallback int) int {
	if errors.Is(err, learning.ErrBudgetExceeded) {
		return http.StatusTooManyRequests
	}
	return fallback
}

func githubErrorStatus(err error) int {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not configured") {
		return http.StatusServiceUnavailable
	}
	if strings.Contains(message, "github url") || strings.Contains(message, "only github.com") || strings.Contains(message, "supported github") {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Service.Settings(r.Context()))
}

func (s *Server) testSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.Service.TestConnections(r.Context())})
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input domain.Settings
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	writeJSON(w, http.StatusOK, s.Service.UpdateSettings(r.Context(), input))
}

func (s *Server) createDailyMission(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkContext string `json:"workContext"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &input); err != nil {
			return
		}
	}
	mission, err := s.Service.CreateDailyMission(r.Context(), input.WorkContext)
	if err != nil {
		writeError(w, featureErrorStatus(err, http.StatusBadGateway), err)
		return
	}
	writeJSON(w, http.StatusCreated, mission)
}

func (s *Server) workContext(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceType string `json:"sourceType"`
		SourceURL  string `json:"sourceUrl"`
		Title      string `json:"title"`
		Content    string `json:"content"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	result, err := s.Service.WorkImportWithURL(r.Context(), input.SourceType, input.Title, input.Content, input.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) missionAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/missions/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "attempts" {
		writeError(w, http.StatusNotFound, errors.New("mission endpoint not found"))
		return
	}
	var input struct {
		Answer string `json:"answer"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}
	result, err := s.Service.SubmitWriting(r.Context(), parts[0], input.Answer)
	if err != nil {
		status := featureErrorStatus(err, http.StatusBadRequest)
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func withCORS(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if len(allowedOrigins) == 0 {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && containsOrigin(allowedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		} else if origin != "" {
			writeError(w, http.StatusForbidden, errors.New("origin is not allowed"))
			return
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func containsOrigin(allowedOrigins []string, origin string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}

func withRequestLog(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.InfoContext(r.Context(), "http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("request body must be valid JSON"))
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": err.Error()}})
}

func WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 30*time.Second)
}
