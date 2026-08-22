-- DevEnglish V1 PostgreSQL schema. The local server starts with a seeded memory store;
-- this migration is the persistence boundary for the next deployment step.
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  cefr TEXT NOT NULL DEFAULT 'A1',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skill_profiles (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  skill TEXT NOT NULL,
  score NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (score BETWEEN 0 AND 100),
  level TEXT NOT NULL DEFAULT 'A1',
  trend NUMERIC(5,2) NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, skill)
);

CREATE TABLE IF NOT EXISTS learning_state (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  state JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS missions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  mode TEXT NOT NULL,
  skill TEXT NOT NULL,
  skill_label TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL,
  context TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  target_vocabulary JSONB NOT NULL DEFAULT '[]'::jsonb,
  expected_points JSONB NOT NULL DEFAULT '[]'::jsonb,
  estimated_minutes INTEGER NOT NULL DEFAULT 10,
  status TEXT NOT NULL DEFAULT 'available',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS mission_attempts (
  id TEXT PRIMARY KEY,
  mission_id TEXT NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  answer TEXT NOT NULL,
  score NUMERIC(5,2) NOT NULL DEFAULT 0,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS mistakes (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  original TEXT NOT NULL,
  corrected TEXT NOT NULL,
  context TEXT NOT NULL DEFAULT '',
  severity SMALLINT NOT NULL DEFAULT 1,
  frequency INTEGER NOT NULL DEFAULT 1,
  last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
  next_review TIMESTAMPTZ NOT NULL DEFAULT now(),
  mastery NUMERIC(5,4) NOT NULL DEFAULT 0 CHECK (mastery BETWEEN 0 AND 1)
);
CREATE INDEX IF NOT EXISTS mistakes_due_idx ON mistakes (user_id, next_review);

CREATE TABLE IF NOT EXISTS vocabulary (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  term TEXT NOT NULL,
  domain TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL DEFAULT 'A1',
  definition TEXT NOT NULL,
  user_context TEXT NOT NULL DEFAULT '',
  technical_example TEXT NOT NULL DEFAULT '',
  related_terms JSONB NOT NULL DEFAULT '[]'::jsonb,
  common_mistakes JSONB NOT NULL DEFAULT '[]'::jsonb,
  mastery NUMERIC(5,4) NOT NULL DEFAULT 0 CHECK (mastery BETWEEN 0 AND 1),
  next_review TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS vocabulary_due_idx ON vocabulary (user_id, next_review);

CREATE TABLE IF NOT EXISTS vocabulary_reviews (
  id BIGSERIAL PRIMARY KEY,
  vocabulary_id TEXT NOT NULL REFERENCES vocabulary(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  success BOOLEAN NOT NULL,
  score NUMERIC(5,2) NOT NULL DEFAULT 0,
  reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skill_history (
  id BIGSERIAL PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  date_key TEXT NOT NULL,
  score NUMERIC(5,2) NOT NULL CHECK (score BETWEEN 0 AND 100),
  UNIQUE (user_id, date_key)
);

CREATE TABLE IF NOT EXISTS diagnostic_results (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  result JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_settings (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  ai_provider TEXT NOT NULL DEFAULT 'deepseek',
  fast_model TEXT NOT NULL DEFAULT 'deepseek-v4-flash',
  smart_model TEXT NOT NULL DEFAULT 'deepseek-v4-pro',
  deepseek_configured BOOLEAN NOT NULL DEFAULT FALSE,
  speech_configured BOOLEAN NOT NULL DEFAULT FALSE,
  pronunciation_on BOOLEAN NOT NULL DEFAULT FALSE,
  monthly_budget_vnd INTEGER NOT NULL DEFAULT 150000
);

CREATE TABLE IF NOT EXISTS work_context (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL,
  source_url TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL,
  domain TEXT NOT NULL DEFAULT '',
  embedding vector(1536),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS work_context_user_created_idx ON work_context (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS work_context_embedding_idx ON work_context USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS imported_sources (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS mistake_occurrences (
  id BIGSERIAL PRIMARY KEY,
  mistake_id TEXT NOT NULL REFERENCES mistakes(id) ON DELETE CASCADE,
  mission_attempt_id TEXT REFERENCES mission_attempts(id) ON DELETE SET NULL,
  observed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS evaluations (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  attempt_id TEXT REFERENCES mission_attempts(id) ON DELETE SET NULL,
  feature TEXT NOT NULL,
  rubric JSONB NOT NULL DEFAULT '{}'::jsonb,
  observation JSONB NOT NULL DEFAULT '{}'::jsonb,
  final_score NUMERIC(5,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS writing_attempts (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mission_id TEXT REFERENCES missions(id) ON DELETE SET NULL,
  answer TEXT NOT NULL,
  evaluation JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS conversations (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  roleplay_type TEXT NOT NULL,
  context TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS messages_conversation_idx ON messages (conversation_id, created_at);

CREATE TABLE IF NOT EXISTS speaking_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mission_id TEXT REFERENCES missions(id) ON DELETE SET NULL,
  audio_uri TEXT,
  transcript TEXT,
  pronunciation JSONB,
  status TEXT NOT NULL DEFAULT 'ready',
  raw_audio_expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_usage (
  id BIGSERIAL PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  feature TEXT NOT NULL,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  audio_seconds NUMERIC(10,2) NOT NULL DEFAULT 0,
  tts_characters INTEGER NOT NULL DEFAULT 0,
  estimated_cost NUMERIC(12,6) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS content_items (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  skill TEXT NOT NULL,
  level TEXT NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  source TEXT NOT NULL DEFAULT 'content-factory',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS prompt_versions (
  id TEXT PRIMARY KEY,
  feature TEXT NOT NULL,
  version TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (feature, version)
);
