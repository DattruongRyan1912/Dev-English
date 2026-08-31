# U0 — User-flow map

    Login
      └─ Today
          ├─ Quick capture task ──> Work task (canonical, version 1)
          ├─ Record decision ─────> Work decision (canonical, version 1)
          ├─ Ask assistant ───────> conversation message
          │                           ├─ grounded + citation
          │                           ├─ inferred/unknown + explanation
          │                           └─ suggested action (preview only)
          └─ English micro-prompt ─> Learning observation (never Work mutation)

    Work project
      ├─ create/update ──> expectedVersion + idempotency key
      ├─ task detail ────> status/priority/next action
      ├─ decision detail ─> context/rationale/outcome/evidence
      └─ conflict/trash ─> explicit resolution or restore

    Knowledge
      ├─ manual import preview ─> immutable revision + chunk/evidence
      ├─ Drive sync ────────────> cursor/checkpoint + sync run
      ├─ GitHub sync ───────────> cursor/checkpoint + sync run
      └─ search ────────────────> FTS/hybrid result + freshness/degraded state

    Learning
      ├─ task/decision mission
      ├─ Vietnamese help ──> English starter ──> editable response
      ├─ push-to-talk ─────> editable transcript ──> submit
      └─ TTS on demand ────> playback only; transcript remains visible

## Acceptance checkpoints

1. A user can create a task without leaving Today or Work.
2. A user can inspect the exact evidence before trusting an assistant answer.
3. A stale/degraded/unknown result is visually distinguishable from verified
   data.
4. A suggested external action cannot execute until a user sees and confirms a
   challenge.
5. A voice failure never removes the editable text/transcript.
6. Learning feedback is observable separately and cannot mutate canonical Work.
