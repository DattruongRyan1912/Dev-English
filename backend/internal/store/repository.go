package store

import (
	"context"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

type userContextKey struct{}

// WithUser attaches the authenticated user identity to a request. Storage
// implementations use it on every query so a future multi-user deployment
// cannot accidentally read another learner's data.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userContextKey{}, userID)
}

func UserID(ctx context.Context) string {
	if value, ok := ctx.Value(userContextKey{}).(string); ok && value != "" {
		return value
	}
	return "user-1"
}

// Repository is the persistence boundary for the modular monolith. Both the
// local seeded store and PostgreSQL implement the same contract so learning
// rules do not depend on storage details.
type Repository interface {
	EnsureUser(context.Context, domain.User) error
	User(context.Context) (domain.User, error)
	LearningState(context.Context) (domain.LearningState, error)
	SaveLearningState(context.Context, domain.LearningState) error
	Mission(context.Context, string) (domain.Mission, error)
	AllMissions(context.Context) ([]domain.Mission, error)
	SaveMission(context.Context, domain.Mission) error
	TodayMission(context.Context) (domain.Mission, error)
	SaveAttempt(context.Context, domain.MissionAttempt) error
	SaveWritingEvaluation(context.Context, domain.MissionAttempt, domain.Evaluation) error
	Mistakes(context.Context, time.Time) ([]domain.Mistake, error)
	AllMistakes(context.Context) ([]domain.Mistake, error)
	UpsertMistake(context.Context, domain.Mistake) error
	ReviewMistake(context.Context, string, bool, float64, time.Time) (domain.Mistake, error)
	Vocabulary(context.Context, time.Time) ([]domain.Vocabulary, error)
	AllVocabulary(context.Context) ([]domain.Vocabulary, error)
	SaveVocabulary(context.Context, domain.Vocabulary) error
	ReviewVocabulary(context.Context, string, bool, float64, time.Time) (domain.Vocabulary, error)
	SaveWorkContext(context.Context, domain.WorkContext) error
	AllWorkContexts(context.Context) ([]domain.WorkContext, error)
	Settings(context.Context) (domain.Settings, error)
	SaveSettings(context.Context, domain.Settings) error
	Progress(context.Context) ([]domain.ProgressPoint, error)
	CompleteMission(context.Context, string, time.Time) error
	UpdateSkill(context.Context, string, float64) error
	SaveDiagnostic(context.Context, domain.DiagnosticResult) error
	Diagnostic(context.Context) (domain.DiagnosticResult, error)
	Scenarios(context.Context) ([]domain.RoleplayScenario, error)
	Conversation(context.Context, string) (domain.Conversation, error)
	SaveConversation(context.Context, domain.Conversation) error
	SpeakingSession(context.Context, string) (domain.SpeakingSession, error)
	AllSpeakingSessions(context.Context) ([]domain.SpeakingSession, error)
	SaveSpeakingSession(context.Context, domain.SpeakingSession) error
	Usage(context.Context, time.Time) ([]domain.UsageRecord, error)
	SaveUsage(context.Context, domain.UsageRecord) error
	DeleteUserData(context.Context) error
}
