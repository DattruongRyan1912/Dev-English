# DevEnglish requirements

Last reviewed: 2026-08-22

## Product objective

Help software developers improve practical English through a focused loop:

`Real Work → AI Simulation → Feedback → Mistake Memory → Next Mission`

The product should turn the developer's technical work into short, repeatable learning sessions instead of a generic English course.

## Core V1 requirements

- Personalised home with a daily mission, current score and weakest skill.
- Diagnostic onboarding with CEFR/skill results and a recommended plan.
- Technical writing mission with structured AI feedback, scoring and retry.
- Mistake extraction and spaced-repetition review.
- Personal technical vocabulary with mastery and related-term graph.
- Practice entry points for writing, speaking, roleplay and learning from work.
- Progress view with learning metrics, trends and weekly speaking assessment.
- Settings view for provider status, pronunciation preference, usage and privacy boundaries.

## AI capabilities

- Text generation/evaluation through the configured DeepSeek provider.
- Speech-to-text through Groq Whisper.
- Pronunciation and prosody assessment through Azure Speech.
- Neural text-to-speech through Azure Speech.
- Deterministic fallback behavior when a provider is unavailable.

## Product and engineering constraints

- Focus-first, mobile-first UI with one clear task per session.
- Backend owns final scores, SRS transitions, mastery, quotas and usage records.
- Provider secrets stay outside source control and are supplied through environment configuration.
- User-scoped authenticated data access, secret-pattern rejection and privacy export/deletion boundaries.
- Local development uses Docker for backend/PostgreSQL and the host for Flutter iteration.

## Source requirements

The original product decisions are recorded in the six source documents in the Google Drive folder `DevEnglish - IT English Learning System`:

1. Product & Learning System
2. Technical Architecture & Data
3. AI Content Factory & Prompt System
4. AI Providers, Voice & Cost Plan
5. MVP Scope & Roadmap
6. UI/UX Design System & Screen Rules

