# DevEnglish U0 — Prototype flow

Status: **flow prototype defined; runtime/device walkthrough required**.

This is the interaction contract for the first rebuilt assistant workflow. It
can be implemented with Flutter routes and bottom sheets before a visual design
tool is introduced; the important property is that every transition preserves
canonical data boundaries.

## Primary path: work to useful English

```text
Today → Next action → Work task detail
     → Ask assistant → answer + evidence + unknowns
     → English starter → editable response → save observation
```

1. Today opens on the highest-priority open task or a truthful empty state.
2. The user opens the task and asks a question in Vietnamese or English.
3. The assistant returns a grounded answer. Evidence is selectable; unknowns
   are explicit; no tool runs from the answer alone.
4. The user requests an English starter or opens the Learning overlay.
5. The user edits the starter/transcript and saves an observation.
6. The observation appears in Learning history and does not alter the task,
   decision or knowledge claim.

## Secondary path: evidence to decision

```text
Knowledge → search → source detail → evidence/claim
          → assistant draft → Work decision preview → confirm → receipt/history
```

The decision write requires the current version and idempotency key. A version
conflict returns the current entity and leaves the original canonical record
unchanged. A confirmed external write uses the separate challenge flow and
shows its receipt in the same UI language as REST/MCP.

## Connector path

```text
Knowledge → Connectors → Drive/GitHub read-only sync
          → run status/cursor → source revision timeline → search
```

Sync is incremental and idempotent. Provider failure leaves the previous
revision/search state intact and marks the run failed or partial with a retry
action. V1 never displays a Drive write control or a GitHub push/merge control.

## Walkthrough matrix

| Walkthrough | Start | Expected visible proof |
| --- | --- | --- |
| Empty workspace | Today with no open task | truthful empty state and create CTA |
| Grounded answer | task + knowledge search | evidence IDs/excerpts and grounding label |
| Unknown answer | question with no evidence | `unknown`, no fabricated citation |
| Work conflict | stale task version | 409/conflict state, reload, no overwrite |
| Safe action | assistant suggested GitHub issue | preview → 5-minute challenge → receipt |
| Learning failure | deny microphone/provider error | editable text path and retry; text preserved |
| Degraded retrieval | embedding sidecar unavailable | FTS results and degraded indicator |
| Connector retry | provider timeout | failed sync run, cursor unchanged, retry |

## Prototype acceptance

- Every path can be completed with text only.
- Back navigation does not lose unsaved text without a warning.
- Loading, empty, error, stale and conflict states are reachable in development
  fixtures.
- Assistant output never mutates Work/Knowledge without a preview and the
  correct confirmation boundary.
- Mobile layout remains usable at 390×844 and desktop layout at 1440×900.
- Operator records approved or requested changes after a real walkthrough.
