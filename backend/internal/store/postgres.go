package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	Pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresStore{Pool: pool}, nil
}

func (s *PostgresStore) Close() { s.Pool.Close() }

func (s *PostgresStore) EnsureUser(ctx context.Context, user domain.User) error {
	if user.ID == "" {
		return errors.New("user id is required")
	}
	if user.DisplayName == "" {
		user.DisplayName = "Developer"
	}
	if user.CEFR == "" {
		user.CEFR = "A1"
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO users (id, display_name, cefr, created_at) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO NOTHING`, user.ID, user.DisplayName, user.CEFR, user.CreatedAt); err != nil {
		return err
	}
	state := defaultLearningState(user.CEFR)
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO learning_state (user_id, state) VALUES ($1,$2) ON CONFLICT (user_id) DO NOTHING`, user.ID, raw); err != nil {
		return err
	}
	for _, skill := range state.Skills {
		if _, err := s.Pool.Exec(ctx, `INSERT INTO skill_profiles (user_id, skill, score, level, trend) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (user_id,skill) DO NOTHING`, user.ID, skill.Skill, skill.Score, skill.Level, skill.Trend); err != nil {
			return err
		}
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO user_settings (user_id, ai_provider, fast_model, smart_model, monthly_budget_vnd) VALUES ($1,'deepseek','deepseek-v4-flash','deepseek-v4-pro',150000) ON CONFLICT (user_id) DO NOTHING`, user.ID)
	return err
}

func (s *PostgresStore) User(ctx context.Context) (domain.User, error) {
	var user domain.User
	err := s.Pool.QueryRow(ctx, `SELECT id, display_name, cefr, created_at FROM users WHERE id = $1`, UserID(ctx)).Scan(&user.ID, &user.DisplayName, &user.CEFR, &user.CreatedAt)
	if err != nil {
		return domain.User{}, mapPGError(err)
	}
	return user, nil
}

func (s *PostgresStore) LearningState(ctx context.Context) (domain.LearningState, error) {
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT state FROM learning_state WHERE user_id = $1`, UserID(ctx)).Scan(&raw)
	if err != nil {
		return domain.LearningState{}, mapPGError(err)
	}
	var state domain.LearningState
	if err := json.Unmarshal(raw, &state); err != nil {
		return domain.LearningState{}, fmt.Errorf("decode learning state: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) SaveLearningState(ctx context.Context, state domain.LearningState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode learning state: %w", err)
	}
	_, err = s.Pool.Exec(ctx, `UPDATE learning_state SET state=$1, updated_at=now() WHERE user_id=$2`, raw, UserID(ctx))
	return err
}

func (s *PostgresStore) Mission(ctx context.Context, id string) (domain.Mission, error) {
	var mission domain.Mission
	var targetRaw, expectedRaw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT id, title, mode, skill, skill_label, level, context, prompt,
		       target_vocabulary, expected_points, estimated_minutes, status, created_at, completed_at
		FROM missions WHERE id = $1 AND user_id = $2`, id, UserID(ctx)).Scan(
		&mission.ID, &mission.Title, &mission.Mode, &mission.Skill, &mission.SkillLabel, &mission.Level,
		&mission.Context, &mission.Prompt, &targetRaw, &expectedRaw, &mission.EstimatedMinutes,
		&mission.Status, &mission.CreatedAt, &mission.CompletedAt,
	)
	if err != nil {
		return domain.Mission{}, mapPGError(err)
	}
	if err := json.Unmarshal(targetRaw, &mission.TargetVocabulary); err != nil {
		return domain.Mission{}, fmt.Errorf("decode mission vocabulary: %w", err)
	}
	if err := json.Unmarshal(expectedRaw, &mission.ExpectedPoints); err != nil {
		return domain.Mission{}, fmt.Errorf("decode mission points: %w", err)
	}
	return mission, nil
}

func (s *PostgresStore) AllMissions(ctx context.Context) ([]domain.Mission, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM missions WHERE user_id=$1 ORDER BY created_at`, UserID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Mission, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		mission, err := s.Mission(ctx, id)
		if err != nil {
			return nil, err
		}
		items = append(items, mission)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SaveMission(ctx context.Context, mission domain.Mission) error {
	target, err := json.Marshal(mission.TargetVocabulary)
	if err != nil {
		return err
	}
	expected, err := json.Marshal(mission.ExpectedPoints)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO missions (id, user_id, title, mode, skill, skill_label, level, context, prompt,
			target_vocabulary, expected_points, estimated_minutes, status, created_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title, mode=EXCLUDED.mode,
			skill=EXCLUDED.skill, skill_label=EXCLUDED.skill_label, level=EXCLUDED.level,
			context=EXCLUDED.context, prompt=EXCLUDED.prompt, target_vocabulary=EXCLUDED.target_vocabulary,
			expected_points=EXCLUDED.expected_points, estimated_minutes=EXCLUDED.estimated_minutes,
			status=EXCLUDED.status, completed_at=EXCLUDED.completed_at
			WHERE missions.user_id=EXCLUDED.user_id`,
		mission.ID, UserID(ctx), mission.Title, mission.Mode, mission.Skill, mission.SkillLabel, mission.Level,
		mission.Context, mission.Prompt, target, expected, mission.EstimatedMinutes, mission.Status,
		mission.CreatedAt, mission.CompletedAt,
	)
	return err
}

func (s *PostgresStore) TodayMission(ctx context.Context) (domain.Mission, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `SELECT id FROM missions WHERE user_id = $1 AND status <> 'completed' ORDER BY created_at DESC LIMIT 1`, UserID(ctx)).Scan(&id)
	if err != nil {
		return domain.Mission{}, mapPGError(err)
	}
	return s.Mission(ctx, id)
}

func (s *PostgresStore) SaveAttempt(ctx context.Context, attempt domain.MissionAttempt) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO mission_attempts (id, mission_id, user_id, answer, score, submitted_at) VALUES ($1,$2,$3,$4,$5,$6)`, attempt.ID, attempt.MissionID, UserID(ctx), attempt.Answer, attempt.Score, attempt.SubmittedAt)
	return err
}

func (s *PostgresStore) SaveWritingEvaluation(ctx context.Context, attempt domain.MissionAttempt, evaluation domain.Evaluation) error {
	raw, err := json.Marshal(evaluation)
	if err != nil {
		return err
	}
	if _, err := s.Pool.Exec(ctx, `INSERT INTO writing_attempts (id, user_id, mission_id, answer, evaluation, created_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING`, attempt.ID, UserID(ctx), attempt.MissionID, attempt.Answer, raw, attempt.SubmittedAt); err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO evaluations (id, user_id, attempt_id, feature, rubric, observation, final_score, created_at) VALUES ($1,$2,$3,'writing',$4,$5,$6,$7) ON CONFLICT (id) DO NOTHING`, "evaluation-"+attempt.ID, UserID(ctx), attempt.ID, `{"grammar":20,"clarity":25,"naturalness":20,"technicalAccuracy":25,"vocabulary":10}`, raw, evaluation.Score, attempt.SubmittedAt)
	return err
}

func (s *PostgresStore) SaveWritingOutcome(ctx context.Context, outcome WritingOutcome) error {
	raw, err := json.Marshal(outcome.Evaluation)
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	userID := UserID(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO mission_attempts (id, mission_id, user_id, answer, score, submitted_at) VALUES ($1,$2,$3,$4,$5,$6)`, outcome.Attempt.ID, outcome.Attempt.MissionID, userID, outcome.Attempt.Answer, outcome.Attempt.Score, outcome.Attempt.SubmittedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO writing_attempts (id, user_id, mission_id, answer, evaluation, created_at) VALUES ($1,$2,$3,$4,$5,$6)`, outcome.Attempt.ID, userID, outcome.Attempt.MissionID, outcome.Attempt.Answer, raw, outcome.Attempt.SubmittedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO evaluations (id, user_id, attempt_id, feature, rubric, observation, final_score, created_at) VALUES ($1,$2,$3,'writing',$4,$5,$6,$7)`, "evaluation-"+outcome.Attempt.ID, userID, outcome.Attempt.ID, `{"grammar":20,"clarity":25,"naturalness":20,"technicalAccuracy":25,"vocabulary":10}`, raw, outcome.Evaluation.Score, outcome.Attempt.SubmittedAt); err != nil {
		return err
	}
	missionResult, err := tx.Exec(ctx, `UPDATE missions SET status='completed', completed_at=$1 WHERE id=$2 AND user_id=$3`, outcome.CompletedAt, outcome.MissionID, userID)
	if err != nil {
		return err
	}
	if missionResult.RowsAffected() == 0 {
		return ErrNotFound
	}
	for _, item := range outcome.Mistakes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO mistakes (id, user_id, type, original, corrected, context, severity, frequency, last_seen, next_review, mastery)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (id) DO UPDATE SET corrected=EXCLUDED.corrected, context=EXCLUDED.context,
				severity=EXCLUDED.severity, frequency=mistakes.frequency+1, last_seen=EXCLUDED.last_seen,
				next_review=EXCLUDED.next_review, mastery=EXCLUDED.mastery
				WHERE mistakes.user_id=EXCLUDED.user_id`, item.ID, userID, item.Type, item.Original, item.Corrected, item.Context, item.Severity, item.Frequency, item.LastSeen, item.NextReview, item.Mastery); err != nil {
			return err
		}
	}
	if err := updateSkillTx(ctx, tx, userID, outcome.Skill, outcome.SkillDelta); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Mistakes(ctx context.Context, now time.Time) ([]domain.Mistake, error) {
	return s.queryMistakes(ctx, `SELECT id, type, original, corrected, context, severity, frequency, last_seen, next_review, mastery FROM mistakes WHERE user_id = $1 AND next_review <= $2 ORDER BY next_review`, UserID(ctx), now)
}

func (s *PostgresStore) AllMistakes(ctx context.Context) ([]domain.Mistake, error) {
	return s.queryMistakes(ctx, `SELECT id, type, original, corrected, context, severity, frequency, last_seen, next_review, mastery FROM mistakes WHERE user_id = $1 ORDER BY mastery`, UserID(ctx))
}

func (s *PostgresStore) queryMistakes(ctx context.Context, query string, args ...any) ([]domain.Mistake, error) {
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Mistake, 0)
	for rows.Next() {
		var item domain.Mistake
		if err := rows.Scan(&item.ID, &item.Type, &item.Original, &item.Corrected, &item.Context, &item.Severity, &item.Frequency, &item.LastSeen, &item.NextReview, &item.Mastery); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) UpsertMistake(ctx context.Context, item domain.Mistake) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO mistakes (id, user_id, type, original, corrected, context, severity, frequency, last_seen, next_review, mastery)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (id) DO UPDATE SET corrected=EXCLUDED.corrected, context=EXCLUDED.context,
			severity=EXCLUDED.severity, frequency=mistakes.frequency+1, last_seen=EXCLUDED.last_seen,
			next_review=EXCLUDED.next_review, mastery=EXCLUDED.mastery
			WHERE mistakes.user_id=EXCLUDED.user_id`,
		item.ID, UserID(ctx), item.Type, item.Original, item.Corrected, item.Context, item.Severity,
		item.Frequency, item.LastSeen, item.NextReview, item.Mastery,
	)
	return err
}

func (s *PostgresStore) ReviewMistake(ctx context.Context, id string, success bool, score float64, now time.Time) (domain.Mistake, error) {
	items, err := s.queryMistakes(ctx, `SELECT id, type, original, corrected, context, severity, frequency, last_seen, next_review, mastery FROM mistakes WHERE user_id = $1 AND id = $2`, UserID(ctx), id)
	if err != nil {
		return domain.Mistake{}, err
	}
	if len(items) == 0 {
		return domain.Mistake{}, ErrNotFound
	}
	item := items[0]
	item.Mastery = updateMastery(item.Mastery, success, score)
	item.LastSeen = now
	item.NextReview = reviewAt(now, item.Mastery, success)
	if !success {
		item.Frequency++
	}
	_, err = s.Pool.Exec(ctx, `UPDATE mistakes SET frequency=$1, last_seen=$2, next_review=$3, mastery=$4 WHERE id=$5 AND user_id=$6`, item.Frequency, item.LastSeen, item.NextReview, item.Mastery, id, UserID(ctx))
	return item, err
}

func (s *PostgresStore) Vocabulary(ctx context.Context, now time.Time) ([]domain.Vocabulary, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, term, domain, level, definition, user_context, technical_example, related_terms, common_mistakes, mastery, next_review FROM vocabulary WHERE user_id = $1 AND next_review <= $2 ORDER BY next_review`, UserID(ctx), now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Vocabulary, 0)
	for rows.Next() {
		var item domain.Vocabulary
		var relatedRaw, commonRaw []byte
		if err := rows.Scan(&item.ID, &item.Term, &item.Domain, &item.Level, &item.Definition, &item.UserContext, &item.TechnicalExample, &relatedRaw, &commonRaw, &item.Mastery, &item.NextReview); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(relatedRaw, &item.RelatedTerms); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(commonRaw, &item.CommonMistakes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) AllVocabulary(ctx context.Context) ([]domain.Vocabulary, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, term, domain, level, definition, user_context, technical_example, related_terms, common_mistakes, mastery, next_review FROM vocabulary WHERE user_id = $1 ORDER BY mastery`, UserID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Vocabulary, 0)
	for rows.Next() {
		var item domain.Vocabulary
		var relatedRaw, commonRaw []byte
		if err := rows.Scan(&item.ID, &item.Term, &item.Domain, &item.Level, &item.Definition, &item.UserContext, &item.TechnicalExample, &relatedRaw, &commonRaw, &item.Mastery, &item.NextReview); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(relatedRaw, &item.RelatedTerms); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(commonRaw, &item.CommonMistakes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SaveVocabulary(ctx context.Context, item domain.Vocabulary) error {
	related, err := json.Marshal(item.RelatedTerms)
	if err != nil {
		return err
	}
	common, err := json.Marshal(item.CommonMistakes)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO vocabulary (id, user_id, term, domain, level, definition, user_context, technical_example, related_terms, common_mistakes, mastery, next_review)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET term=EXCLUDED.term, domain=EXCLUDED.domain, level=EXCLUDED.level,
			definition=EXCLUDED.definition, user_context=EXCLUDED.user_context, technical_example=EXCLUDED.technical_example,
			related_terms=EXCLUDED.related_terms, common_mistakes=EXCLUDED.common_mistakes, mastery=EXCLUDED.mastery, next_review=EXCLUDED.next_review
			WHERE vocabulary.user_id=EXCLUDED.user_id`,
		item.ID, UserID(ctx), item.Term, item.Domain, item.Level, item.Definition, item.UserContext, item.TechnicalExample,
		related, common, item.Mastery, item.NextReview)
	return err
}

func (s *PostgresStore) ReviewVocabulary(ctx context.Context, id string, success bool, score float64, now time.Time) (domain.Vocabulary, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, term, domain, level, definition, user_context, technical_example, related_terms, common_mistakes, mastery, next_review FROM vocabulary WHERE user_id=$1 AND id=$2`, UserID(ctx), id)
	if err != nil {
		return domain.Vocabulary{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.Vocabulary{}, ErrNotFound
	}
	item := domain.Vocabulary{}
	var relatedRaw, commonRaw []byte
	if err := rows.Scan(&item.ID, &item.Term, &item.Domain, &item.Level, &item.Definition, &item.UserContext, &item.TechnicalExample, &relatedRaw, &commonRaw, &item.Mastery, &item.NextReview); err != nil {
		return domain.Vocabulary{}, err
	}
	if err := json.Unmarshal(relatedRaw, &item.RelatedTerms); err != nil {
		return domain.Vocabulary{}, err
	}
	if err := json.Unmarshal(commonRaw, &item.CommonMistakes); err != nil {
		return domain.Vocabulary{}, err
	}
	item.Mastery = updateMastery(item.Mastery, success, score)
	item.NextReview = reviewAt(now, item.Mastery, success)
	_, err = s.Pool.Exec(ctx, `UPDATE vocabulary SET mastery=$1, next_review=$2 WHERE id=$3 AND user_id=$4`, item.Mastery, item.NextReview, id, UserID(ctx))
	if err != nil {
		return domain.Vocabulary{}, err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO vocabulary_reviews (vocabulary_id, user_id, success, score, reviewed_at) VALUES ($1,$2,$3,$4,$5)`, id, UserID(ctx), success, score, now)
	return item, err
}

func (s *PostgresStore) SaveWorkContext(ctx context.Context, item domain.WorkContext) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO work_context (id, user_id, source_type, source_url, title, content, domain, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, UserID(ctx), item.SourceType, item.SourceURL, item.Title, item.Content, item.Domain, item.CreatedAt)
	return err
}

func (s *PostgresStore) AllWorkContexts(ctx context.Context) ([]domain.WorkContext, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, source_type, source_url, title, content, domain, created_at FROM work_context WHERE user_id=$1 ORDER BY created_at`, UserID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.WorkContext, 0)
	for rows.Next() {
		var item domain.WorkContext
		if err := rows.Scan(&item.ID, &item.SourceType, &item.SourceURL, &item.Title, &item.Content, &item.Domain, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) Settings(ctx context.Context) (domain.Settings, error) {
	var settings domain.Settings
	err := s.Pool.QueryRow(ctx, `SELECT ai_provider, fast_model, smart_model, deepseek_configured, deepseek_status, speech_configured, pronunciation_on, monthly_budget_vnd FROM user_settings WHERE user_id = $1`, UserID(ctx)).Scan(&settings.AIProvider, &settings.FastModel, &settings.SmartModel, &settings.DeepSeekConfigured, &settings.DeepSeekStatus, &settings.SpeechConfigured, &settings.PronunciationOn, &settings.MonthlyBudgetVND)
	if err != nil {
		return domain.Settings{}, mapPGError(err)
	}
	return settings, nil
}

func (s *PostgresStore) SaveSettings(ctx context.Context, settings domain.Settings) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO user_settings (user_id, ai_provider, fast_model, smart_model, deepseek_configured, deepseek_status, speech_configured, pronunciation_on, monthly_budget_vnd)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (user_id) DO UPDATE SET ai_provider=EXCLUDED.ai_provider, fast_model=EXCLUDED.fast_model,
			smart_model=EXCLUDED.smart_model, deepseek_configured=EXCLUDED.deepseek_configured,
			deepseek_status=EXCLUDED.deepseek_status,
			speech_configured=EXCLUDED.speech_configured, pronunciation_on=EXCLUDED.pronunciation_on,
			monthly_budget_vnd=EXCLUDED.monthly_budget_vnd`,
		UserID(ctx), settings.AIProvider, settings.FastModel, settings.SmartModel, settings.DeepSeekConfigured, settings.DeepSeekStatus, settings.SpeechConfigured, settings.PronunciationOn, settings.MonthlyBudgetVND,
	)
	return err
}

func (s *PostgresStore) DeepSeekSecret(ctx context.Context) (EncryptedSecret, error) {
	var secret EncryptedSecret
	err := s.Pool.QueryRow(ctx, `SELECT ciphertext, nonce FROM provider_secrets WHERE user_id=$1 AND provider='deepseek'`, UserID(ctx)).Scan(&secret.Ciphertext, &secret.Nonce)
	if err != nil {
		return EncryptedSecret{}, mapPGError(err)
	}
	return secret, nil
}

func (s *PostgresStore) SaveDeepSeekSecret(ctx context.Context, secret EncryptedSecret) error {
	if len(secret.Ciphertext) == 0 || len(secret.Nonce) == 0 {
		return errors.New("encrypted DeepSeek secret is required")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO provider_secrets (user_id, provider, ciphertext, nonce, updated_at)
		VALUES ($1,'deepseek',$2,$3,now())
		ON CONFLICT (user_id, provider) DO UPDATE SET ciphertext=EXCLUDED.ciphertext, nonce=EXCLUDED.nonce, updated_at=now()`,
		UserID(ctx), secret.Ciphertext, secret.Nonce,
	)
	return err
}

func (s *PostgresStore) DeleteDeepSeekSecret(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM provider_secrets WHERE user_id=$1 AND provider='deepseek'`, UserID(ctx))
	return err
}

func (s *PostgresStore) Progress(ctx context.Context) ([]domain.ProgressPoint, error) {
	rows, err := s.Pool.Query(ctx, `SELECT date_key, score FROM skill_history WHERE user_id = $1 ORDER BY date_key ASC LIMIT 30`, UserID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ProgressPoint, 0)
	for rows.Next() {
		var point domain.ProgressPoint
		if err := rows.Scan(&point.Date, &point.Score); err != nil {
			return nil, err
		}
		items = append(items, point)
	}
	return items, rows.Err()
}

func (s *PostgresStore) CompleteMission(ctx context.Context, id string, completedAt time.Time) error {
	result, err := s.Pool.Exec(ctx, `UPDATE missions SET status='completed', completed_at=$1 WHERE id=$2 AND user_id=$3`, completedAt, id, UserID(ctx))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) UpdateSkill(ctx context.Context, skill string, delta float64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := updateSkillTx(ctx, tx, UserID(ctx), skill, delta); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func updateSkillTx(ctx context.Context, tx pgx.Tx, userID, skill string, delta float64) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT state FROM learning_state WHERE user_id=$1 FOR UPDATE`, userID).Scan(&raw); err != nil {
		return mapPGError(err)
	}
	var state domain.LearningState
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("decode learning state: %w", err)
	}
	for index := range state.Skills {
		if state.Skills[index].Skill == skill {
			state.Skills[index].Score = clampScore(state.Skills[index].Score + delta)
			if delta > 0 {
				state.Skills[index].Trend += delta
			}
		}
	}
	state = recomputeLearningState(state)
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE skill_profiles SET score=LEAST(100, GREATEST(0, score+$1)), trend=trend+GREATEST(0,$1), updated_at=now() WHERE user_id=$2 AND skill=$3`, delta, userID, skill); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE learning_state SET state=$1, updated_at=now() WHERE user_id=$2`, raw, userID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO skill_history (user_id, date_key, score) VALUES ($1, $2, $3) ON CONFLICT (user_id,date_key) DO UPDATE SET score=EXCLUDED.score`, userID, time.Now().UTC().Format("2006-01-02"), state.OverallScore)
	return err
}

func (s *PostgresStore) SaveDiagnostic(ctx context.Context, result domain.DiagnosticResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO diagnostic_results (user_id, result) VALUES ($1,$2) ON CONFLICT (user_id) DO UPDATE SET result=EXCLUDED.result, created_at=now()`, UserID(ctx), raw)
	return err
}

func (s *PostgresStore) Diagnostic(ctx context.Context) (domain.DiagnosticResult, error) {
	var raw []byte
	if err := s.Pool.QueryRow(ctx, `SELECT result FROM diagnostic_results WHERE user_id=$1`, UserID(ctx)).Scan(&raw); err != nil {
		return domain.DiagnosticResult{}, mapPGError(err)
	}
	var result domain.DiagnosticResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return domain.DiagnosticResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) Scenarios(context.Context) ([]domain.RoleplayScenario, error) {
	return defaultScenarios(), nil
}

func (s *PostgresStore) Conversation(ctx context.Context, id string) (domain.Conversation, error) {
	var conversation domain.Conversation
	if err := s.Pool.QueryRow(ctx, `SELECT id, roleplay_type, context, status, created_at FROM conversations WHERE id=$1 AND user_id=$2`, id, UserID(ctx)).Scan(&conversation.ID, &conversation.RoleplayType, &conversation.Context, &conversation.Status, &conversation.CreatedAt); err != nil {
		return domain.Conversation{}, mapPGError(err)
	}
	rows, err := s.Pool.Query(ctx, `SELECT id, role, content, created_at FROM messages WHERE conversation_id=$1 ORDER BY created_at, id`, id)
	if err != nil {
		return domain.Conversation{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(&message.ID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return domain.Conversation{}, err
		}
		conversation.Messages = append(conversation.Messages, message)
	}
	return conversation, rows.Err()
}

func (s *PostgresStore) SaveConversation(ctx context.Context, conversation domain.Conversation) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `
		INSERT INTO conversations (id, user_id, roleplay_type, context, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET roleplay_type=EXCLUDED.roleplay_type, context=EXCLUDED.context, status=EXCLUDED.status
		WHERE conversations.user_id=EXCLUDED.user_id`, conversation.ID, UserID(ctx), conversation.RoleplayType, conversation.Context, conversation.Status, conversation.CreatedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM messages WHERE conversation_id=$1`, conversation.ID); err != nil {
		return err
	}
	for _, message := range conversation.Messages {
		if _, err := tx.Exec(ctx, `INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES ($1,$2,$3,$4,$5)`, message.ID, conversation.ID, message.Role, message.Content, message.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) SpeakingSession(ctx context.Context, id string) (domain.SpeakingSession, error) {
	var session domain.SpeakingSession
	var missionID *string
	var transcript *string
	var raw []byte
	if err := s.Pool.QueryRow(ctx, `SELECT id, mission_id, status, transcript, pronunciation, raw_audio_expires_at, created_at FROM speaking_sessions WHERE id=$1 AND user_id=$2`, id, UserID(ctx)).Scan(&session.ID, &missionID, &session.Status, &transcript, &raw, &session.RawAudioExpiresAt, &session.CreatedAt); err != nil {
		return domain.SpeakingSession{}, mapPGError(err)
	}
	if missionID != nil {
		session.MissionID = *missionID
	}
	if transcript != nil {
		session.Transcript = *transcript
	}
	if len(raw) > 0 {
		var score domain.PronunciationScore
		if err := json.Unmarshal(raw, &score); err != nil {
			return domain.SpeakingSession{}, err
		}
		session.Pronunciation = &score
	}
	return session, nil
}

func (s *PostgresStore) AllSpeakingSessions(ctx context.Context) ([]domain.SpeakingSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, mission_id, status, transcript, pronunciation, raw_audio_expires_at, created_at FROM speaking_sessions WHERE user_id=$1 ORDER BY created_at`, UserID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.SpeakingSession, 0)
	for rows.Next() {
		var session domain.SpeakingSession
		var missionID *string
		var transcript *string
		var raw []byte
		if err := rows.Scan(&session.ID, &missionID, &session.Status, &transcript, &raw, &session.RawAudioExpiresAt, &session.CreatedAt); err != nil {
			return nil, err
		}
		if missionID != nil {
			session.MissionID = *missionID
		}
		if transcript != nil {
			session.Transcript = *transcript
		}
		if len(raw) > 0 {
			var score domain.PronunciationScore
			if err := json.Unmarshal(raw, &score); err != nil {
				return nil, err
			}
			session.Pronunciation = &score
		}
		items = append(items, session)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SaveSpeakingSession(ctx context.Context, session domain.SpeakingSession) error {
	var pronunciation []byte
	var err error
	if session.Pronunciation != nil {
		pronunciation, err = json.Marshal(session.Pronunciation)
		if err != nil {
			return err
		}
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO speaking_sessions (id, user_id, mission_id, transcript, pronunciation, status, raw_audio_expires_at, created_at)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET mission_id=EXCLUDED.mission_id, transcript=EXCLUDED.transcript,
			pronunciation=EXCLUDED.pronunciation, status=EXCLUDED.status, raw_audio_expires_at=EXCLUDED.raw_audio_expires_at`,
		session.ID, UserID(ctx), session.MissionID, session.Transcript, pronunciation, session.Status, session.RawAudioExpiresAt, session.CreatedAt)
	return err
}

func (s *PostgresStore) Usage(ctx context.Context, from time.Time) ([]domain.UsageRecord, error) {
	rows, err := s.Pool.Query(ctx, `SELECT provider, model, feature, input_tokens, output_tokens, audio_seconds, tts_characters, estimated_cost, created_at FROM ai_usage WHERE user_id=$1 AND created_at >= $2 ORDER BY created_at DESC`, UserID(ctx), from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.UsageRecord, 0)
	for rows.Next() {
		var item domain.UsageRecord
		if err := rows.Scan(&item.Provider, &item.Model, &item.Feature, &item.InputTokens, &item.OutputTokens, &item.AudioSeconds, &item.TTSCharacters, &item.EstimatedCost, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) SaveUsage(ctx context.Context, record domain.UsageRecord) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO ai_usage (user_id, provider, model, feature, input_tokens, output_tokens, audio_seconds, tts_characters, estimated_cost, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, UserID(ctx), record.Provider, record.Model, record.Feature, record.InputTokens, record.OutputTokens, record.AudioSeconds, record.TTSCharacters, record.EstimatedCost, record.CreatedAt)
	return err
}

func (s *PostgresStore) DeleteUserData(ctx context.Context) error {
	result, err := s.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, UserID(ctx))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsureLocalSeed makes a fresh local database usable without coupling the
// server to the in-memory implementation. Seed content can be expanded by the
// content factory later without changing this persistence boundary.
func (s *PostgresStore) EnsureLocalSeed(ctx context.Context) error {
	userID := UserID(ctx)
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id, display_name, cefr) VALUES ($1,'Developer','B1') ON CONFLICT (id) DO NOTHING`, userID)
	if err != nil {
		return err
	}
	state := domain.LearningState{CEFR: "B1", OverallScore: 58, WeakestSkill: "technical_writing", RecentTopics: []string{"API error handling", "Flutter state", "PostgreSQL"}, CurrentPriorities: []string{"write clearer bug reports", "explain root cause", "review technical vocabulary"}, Skills: []domain.SkillState{{Skill: "documentation", Label: "Documentation", Score: 66, Level: "B1", Trend: 4}, {Skill: "technical_writing", Label: "Technical Writing", Score: 48, Level: "B1", Trend: 2, IsWeakest: true}, {Skill: "git_communication", Label: "Git Communication", Score: 54, Level: "B1", Trend: 3}, {Skill: "speaking", Label: "Speaking", Score: 57, Level: "B1", Trend: 5}, {Skill: "meeting", Label: "Meeting", Score: 61, Level: "B1", Trend: 1}, {Skill: "system_design", Label: "System Design", Score: 45, Level: "A2"}}}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO learning_state (user_id, state) VALUES ($1,$2) ON CONFLICT (user_id) DO NOTHING`, userID, raw)
	if err != nil {
		return err
	}
	for _, skill := range state.Skills {
		if _, err := s.Pool.Exec(ctx, `INSERT INTO skill_profiles (user_id, skill, score, level, trend) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (user_id,skill) DO NOTHING`, userID, skill.Skill, skill.Score, skill.Level, skill.Trend); err != nil {
			return err
		}
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO user_settings (user_id, ai_provider, fast_model, smart_model, monthly_budget_vnd) VALUES ($1,'deepseek','deepseek-v4-flash','deepseek-v4-pro',150000) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}

func clampScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func recomputeLearningState(state domain.LearningState) domain.LearningState {
	if len(state.Skills) == 0 {
		return state
	}
	weakest := 0
	total := 0.0
	for index := range state.Skills {
		state.Skills[index].IsWeakest = false
		total += state.Skills[index].Score
		if state.Skills[index].Score < state.Skills[weakest].Score {
			weakest = index
		}
	}
	state.Skills[weakest].IsWeakest = true
	state.WeakestSkill = state.Skills[weakest].Skill
	state.OverallScore = total / float64(len(state.Skills))
	return state
}

func defaultLearningState(cefr string) domain.LearningState {
	if cefr == "" {
		cefr = "B1"
	}
	return domain.LearningState{
		CEFR:              cefr,
		OverallScore:      58,
		WeakestSkill:      "technical_writing",
		RecentTopics:      []string{"API error handling", "Flutter state", "PostgreSQL"},
		CurrentPriorities: []string{"write clearer bug reports", "explain root cause", "review technical vocabulary"},
		Skills: []domain.SkillState{
			{Skill: "documentation", Label: "Documentation", Score: 66, Level: "B1", Trend: 4, Description: "Read and summarize technical docs."},
			{Skill: "technical_writing", Label: "Technical Writing", Score: 48, Level: "B1", Trend: 2, IsWeakest: true, Description: "Write clear developer-facing messages."},
			{Skill: "git_communication", Label: "Git Communication", Score: 54, Level: "B1", Trend: 3, Description: "Write commits, issues and PRs."},
			{Skill: "speaking", Label: "Speaking", Score: 57, Level: "B1", Trend: 5, Description: "Explain code and technical decisions."},
			{Skill: "meeting", Label: "Meeting", Score: 61, Level: "B1", Trend: 1, Description: "Clarify requirements and discuss trade-offs."},
			{Skill: "system_design", Label: "System Design", Score: 45, Level: "A2", Trend: 0, Description: "Describe architecture and failure handling."},
		},
	}
}

func mapPGError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
