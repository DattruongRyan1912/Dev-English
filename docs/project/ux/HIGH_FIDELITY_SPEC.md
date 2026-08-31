# DevEnglish U0 — High-fidelity specification

Status: **implementation reference; operator/device review still required**.

This document turns the U0 audit, flow map, wireframes and token sheet into a
buildable visual contract. It is intentionally specific enough to produce the
same UI on Flutter web, Android and a narrow desktop window without turning the
old learning shell into another card wall.

## Product posture

DevEnglish is an editorial command center for a developer's real work. English
is a quiet overlay on that work: it explains, rewrites and rehearses useful
language, but it must never rewrite canonical work or knowledge data.

The visual hierarchy is therefore:

1. one concrete next action;
2. the evidence or source behind it;
3. a short assistant response and an explicit action boundary;
4. optional English practice.

Avoid gradients, decorative dashboards, fake progress, dense chip clouds and
generic “AI generated” copy. Every state must explain what is happening and
what the user can do next.

## Viewports and layout

Golden targets:

| Target | Layout contract |
| --- | --- |
| Mobile 390×844 | Safe-area top bar, one-column content, fixed bottom navigation, no horizontal scroll |
| Mobile 430×932 | Same content model; wider reading measure, no extra dashboard columns |
| Desktop 1440×900 | 240px navigation rail, 680–760px primary reading column, optional 300px evidence/action inspector |
| Narrow desktop 900–1199px | Collapse rail to icon+label drawer; inspector becomes a bottom sheet |

Mobile content uses 16px side padding, 24px section spacing and a 68px bottom
safe-area reservation. Desktop content uses a 24px outer gutter and a maximum
reading measure of 760px. Interactive controls have at least a 44×44 logical
tap target.

## Surface specifications

### Today

- Header: workspace name, sync/status affordance and settings; no greeting that
  consumes the first screenful.
- `Next action` is the first prominent block. It contains source, age/priority,
  one-sentence intent and a single primary CTA.
- `Briefing` shows at most three facts and their source labels.
- `Ask` is a text-first composer with a visible “uses workspace data” hint.
- `Learning overlay` is collapsed by default and can offer one English starter
  related to the current task.
- Empty state: “No open work yet” plus create-task CTA; never an invented task.

### Work

- Projects, tasks and decisions are represented as a compact list/table with
  status, owner, updated time and version.
- Create/edit is an explicit form or command preview. Save responses show the
  resulting version and receipt/history link.
- Conflicts show expected/current version and offer reload; they never silently
  overwrite.
- Trash is a secondary filter. Restore and purge require clear destructive
  copy, with purge separated from ordinary save.

### Knowledge

- Search starts exact-first and visibly labels `grounded`, `inferred`,
  `unknown` and `degraded` states.
- Results show title, source, revision timestamp and evidence excerpt. A claim
  without evidence cannot be presented as a fact.
- Source detail is a timeline: source item → immutable revision → chunks →
  claims/evidence. Stale sources remain visible as stale.
- Drive/GitHub controls are read-only in V1 and show the last sync cursor/run.

### Assistant / conversation

- Conversation is a reading surface, not a floating chatbot bubble.
- Each response has answer, grounding label, evidence links, unknowns and
  suggested actions. Suggested actions are previews, never implicit execution.
- External write actions show target, canonical action hash context and a
  confirm step; the final receipt remains visible in the thread.
- The composer supports Vietnamese help and keeps an English starter beside
  the explanation when learning mode is active.

### Learning

- Learning is work-derived: mission prompt, editable response/transcript,
  short feedback and one next practice step.
- TTS is opt-in. Push-to-talk produces editable text before submission.
- An observation is clearly separate from Work/Knowledge and can be discarded
  without changing canonical data.

## Component states

Every primary surface must cover these states in implementation and screenshots:

| Component | Required states |
| --- | --- |
| Async list | loading, populated, empty, stale, degraded, error, retrying |
| Assistant answer | grounded, inferred, unknown, provider error, quota exhausted |
| Work mutation | draft, preview, saving, success with version/receipt, conflict, validation error |
| Connector | never connected, syncing, cursor advanced, no changes, partial failure, read-only |
| Learning input | idle, recording, editable transcript, submitting, saved, permission/error |
| Navigation | active, keyboard focus, disabled-by-scope, compact mobile |

## Content and accessibility

- Primary UI copy is plain English; Vietnamese help is short and appears on
  request or when the user signals uncertainty.
- Use sentence case, concrete verbs and no unexplained provider jargon.
- Preserve text transcripts even when audio/STT/TTS fails.
- Semantics labels must identify icon-only controls; focus order follows the
  reading order; keyboard can open/close sheets and submit forms.
- Contrast must meet WCAG AA for body text and focus indicators must remain
  visible on light and dark themes.

## Evidence required before U0 acceptance

- screenshots/goldens at 390×844, 430×932 and 1440×900 for Today, Work,
  Knowledge, Assistant and Learning;
- one capture for each error/empty/stale/conflict state in the matrix above;
- a device/browser review of tap targets, keyboard focus, text overflow and
  safe-area behavior;
- operator verdict recorded in `OPERATOR_VERDICT.md`.

The current Flutter implementation is a functional walking skeleton and is
mapped to these surfaces incrementally. This specification does not claim the
operator/device review is complete.
