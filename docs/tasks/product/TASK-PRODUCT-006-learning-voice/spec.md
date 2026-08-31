---
id: TASK-PRODUCT-006
title: Add workflow-derived Learning overlay and text-first voice controls
template: task@1
type: feature
module: product
status: on_hold
priority: p1
author: "@codex"
department: engineering
created_at: 2026-08-28T21:35:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: limited
client_visible: false
depends_on: [TASK-PRODUCT-005]
new_files:
  - backend/internal/learningoverlay/
  - lib/src/features/learning/workspace_learning.dart
  - test/roleplay_test.dart
  - test/workspace_state_golden_test.dart
modified_files:
  - backend/internal/learning/
  - backend/internal/ai/speech.go
  - lib/src/screens/roleplay_screen.dart
  - lib/src/screens/speaking_screen.dart
---

# TASK-PRODUCT-006: Add workflow-derived Learning overlay and text-first voice controls

> Lifecycle hold: wait for `TASK-PRODUCT-005` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Turn real work conversations into optional English practice while keeping text
transcripts editable and making speech input/output explicit user actions.

## Problem

The old learning features are disconnected from work context, and generated
English responses can be too advanced for the user's current level.

## Proposed Solution

Add a learning overlay that explains briefly in Vietnamese, offers an English
starter and a follow-up, with optional Groq STT, Azure TTS/pronunciation and
editable transcript persistence.

## Alternatives Considered

- Force every work conversation into scored practice. Rejected because work assistance must remain primary.
- Hide the transcript after speech recognition. Rejected because text recognition and correction are part of the learning value.

## Success Metrics

- A user can ask for help answering and receive level-aware bilingual support.
- STT text remains editable before submission and TTS runs only after a tap.
- Learning observations never mutate canonical work data.

## Scope

In scope: overlay guidance, workflow-derived missions, roleplay/speaking
integration, editable transcript and on-demand TTS.

Out of scope: realtime full-duplex voice and autonomous scoring of ordinary work chat.

## Dependencies

- TASK-PRODUCT-005 assistant conversation contract.
- Groq STT and Azure Speech configuration.
- Existing SRS, vocabulary, mistakes and roleplay logic.

## 1. Description

- Present English as an optional overlay on the current work context.
- Keep user-visible transcript text as the durable representation of speech input.
- Queue learning observations separately from canonical work writes.

## Acceptance criteria

- [ ] AC 1 — Vietnamese help produces an English starter and one follow-up at the user's level. (test: `backend/internal/ai/provider_test.go::TestDeterministicRoleplayAdaptsToVietnameseHelp`)
- [ ] AC 2 — Transcript remains editable and TTS is on-demand in the learning UI. (test: `backend/internal/httpapi/server_test.go::TestSpeechHTTPEndpointsPreserveEditableTranscriptFlow`; `test/workspace_ui_test.dart::learning support opens and hold-to-talk tracks press lifecycle`)
- [ ] AC 3 — Learning observations are isolated from Work and Knowledge persistence. (test: `backend/internal/httpapi/server_test.go::TestV2LearningOverlayRecordsWithoutMutatingWork`; `backend/internal/learningoverlay/service_test.go::TestServiceRecordsAndListsOnlyWithinScope`)

## AI Risk Assessment

### Data Sources

The overlay uses the current workflow context, user-provided transcript and
learning profile; it does not promote generated text to canonical work data.

### Human Oversight

The user edits and submits transcripts, chooses TTS playback and can ignore the overlay.

### Failure Modes

Provider outage, low-confidence transcript and over-advanced wording degrade to
visible text states and a short bilingual explanation.

## Protected invariants

- Text remains the primary record even when speech is used.
- Learning data remains separate from canonical Work/Knowledge records.
- No realtime audio stream is required for V1.

## AI Authorship Disclosure

- Tools used: Codex with Go learning/speech tests and Flutter roleplay tests.
- Scope: re-derived and CONFIRMED: text-first learning behavior; re-derived and CORRECTED: overlay versus canonical data separation; measured and ADDED: transcript/edit/TTS evidence.
- Human review: the operator reviews the bilingual overlay and mobile transcript flow.
