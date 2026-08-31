# Product reset implementation evidence — 2026-08-28

## 2026-08-29 continuation — coverage and lifecycle boundary refresh

- Go unit suite: `526` tests passed across 23 packages; race suite: `526`
  tests passed; vet/build exited `0`.
- DB-enabled coverage run: `540` tests passed with `67.8%` total statement
  coverage. Application is `81.3%`, Knowledge `82.1%`, MCP `82.0%`, Work
  `84.5%`, Connectors `72.2%` and HTTP API `62.7%`.
- Added HTTP boundary evidence for active-child purge rejection and early
  purge retention errors. The canonical records remain intact in both cases.
- The plan's `90%` coverage threshold, U0/lifecycle acceptance and release
  authorization remain open.

## 2026-08-29 continuation — MCP identity repair and final local gates

This continuation stayed in the canonical WIP checkout and preserved unrelated
changes. It did not commit, push, merge or deploy. The source repair bound
full-application MCP writes for Project, Task, Decision and manual Knowledge
import to the authenticated token principal, and added boundary tests for
workspace isolation, replay, malformed input, foreign Knowledge references and
soft-delete parity.

Observed local evidence:

- Go tests: 494 passed across 23 packages (historical checkpoint).
- Go race tests: 494 passed; vet and build passed (historical checkpoint).
- Flutter analyze passed; Flutter tests passed with 43 passed and 2 skipped;
  the web release build passed.
- Production-shaped disposable smoke passed with 18 migrations, all
  Project/Task/Decision Work conflict checks, MCP token lifecycle, Knowledge
  import/search, CORS, provider fail-closed and logout checks.
- That historical profile measured 68.3%; the current DB-enabled refresh is
  recorded above and measures 67.8% overall. The plan threshold of 90% remains
  open.

## 2026-08-29 — Wave 3 canonical feature boundary slice

The Flutter implementation now has a public feature composition boundary at
`lib/src/features/workspace/workspace.dart`. The app shell imports the public
Today, Work, Knowledge and Learning libraries directly; the former
`screens/workspace_screen.dart` is retained as a compatibility facade for the
internal part-file implementation. This slice changes no runtime behavior and
was verified with Flutter analyze, 43 tests with 2 skipped, a release web build,
and a visual browser smoke that rendered Today and Work from `build/web` at
`http://localhost:8094/`. The browser semantic tree exposed only Flutter's
accessibility placeholder, so the smoke evidence is visual and not a DOM
semantic assertion.

Review boundary:

- The previous bounded Sol Ultra repair review remains approved.
- The latest bounded Sol Ultra review attempt terminated at the usage limit
  before a verdict; no approval or rejection is inferred from that attempt.
- U0 operator/device acceptance, lifecycle acceptance and human authorization
  for Git/release operations remain open.

## Scope

This run continued the approved DevEnglish product-reset plan from the existing
WIP checkout. It covered the S5–S7 implementation boundary and the reviewer
repair follow-up without resetting, cleaning, committing, pushing, merging or
deploying the checkout.

The implementation now has a runnable path across:

```text
Today/Work/Knowledge/Learning
    -> PostgreSQL-backed application services
    -> grounded assistant and usage guard
    -> challenge/receipt action boundary
    -> MCP token/resources/tools
    -> production-shaped Docker smoke
```

## Source changes covered by this evidence

- Added durable, PostgreSQL-backed pending/complete safe-write reservations so
  an external provider success followed by receipt persistence failure is
  treated as `uncertain` and cannot be replayed blindly.
- Added digest-only persistent MCP tokens, expiry/revoke state and restart
  semantics, with migration `016_mcp_tokens.sql` and server wiring.
- Added full initial Drive bootstrap, removal/tombstone handling and
  workspace-scoped connector revision/item/chunk identity.
- Added explicit GitHub restart-cursor persistence and retained the existing
  read-only/safe-write boundaries.
- Added the product-reset task graph `TASK-PRODUCT-001` through
  `TASK-PRODUCT-008` under `docs/tasks/product/`; `TASK-PRODUCT-001` and OPS
  are `ready_to_review`, while successors 002–008 are `on_hold` behind the
  sequential human lifecycle gates.
- Added production-shaped MCP token issue → initialize → revoke smoke coverage
  without logging bearer values.
- Repaired manual-source retry identity so repeating the same import reuses the
  immutable revision, does not create a timestamp-only revision, and does not
  bump the source-item version when the current revision is already selected.
- Scoped safe-write receipt lookup and deterministic receipt identity by
  workspace, added Drive trashed-file tombstone/restore handling, and added an
  opt-in PostgreSQL application-level idempotency integration test.

## Verified commands

| Gate | Exit | Observed result |
| --- | ---: | --- |
| `rtk go test ./...` | 0 | 438 tests passed across 23 packages |
| `rtk go test -race ./...` | 0 | 438 tests passed across 23 packages |
| `rtk go vet ./...` | 0 | No issues |
| `rtk go build ./...` | 0 | Build succeeded |
| `rtk dart format --output=none --set-exit-if-changed lib test` | 0 | Final rerun clean after formatting `lib/main.dart` |
| `rtk flutter analyze` | 0 | No issues found |
| `rtk flutter test --reporter compact` | 0 | 43 passed, 2 skipped |
| `rtk git diff --check` | 0 | Clean whitespace check |
| `rtk node .cyberos/docs-tools/task-lint.mjs docs/tasks` | 0 | 9 trace/info messages, no errors |
| `rtk bash scripts/verify_multi_agent_protocol.sh` | 0 | Entrypoints, manifests and protocol checks passed |
| `rtk bash -n scripts/production_runtime_smoke.sh scripts/verify_cyberos_only.sh scripts/verify_multi_agent_protocol.sh` | 0 | Shell syntax clean |
| `DEVENGLISH_TEST_DATABASE_URL=... rtk go test ./backend/internal/application -run TestPostgresApplicationManualImportIsIdempotent -count=1` | 0 | PostgreSQL manual import retry preserved one revision/chunk and the source-item version |
| `rtk flutter build web --release` + browser smoke at `http://localhost:8094/` | 0 | Release artifact rendered the canonical Today view; no browser error/warning entries |

The production-shaped runtime command was:

```text
rtk env DEVENGLISH_PRODUCTION_SMOKE=YES ./scripts/production_runtime_smoke.sh
```

It exited `0` with:

```text
migrations=17, embedding=200, login=200, session=200,
mcp-issue=201, mcp-initialize=200, mcp-revoke=204, mcp-revoked=401,
knowledge-import=201, knowledge-search=200, cors=204,
forbidden-cors=403, provider-fail-closed=500, logout=204
```

The disposable PostgreSQL/backend/embedding stack was cleaned up by the
script; existing development containers were not targeted.

## Coverage boundary

- Current all-backend profile: `55.5%` statement coverage.
- Database-only paths require the configured integration fixture to contribute
  coverage; the plan threshold remains `90%`.
- The plan threshold is `90%`; this is still an open acceptance item. No
  release approval is inferred from the passing tests.
- The PostgreSQL MCP persistence integration test is available but skips when
  `DEVENGLISH_TEST_DATABASE_URL` is unset. The production-shaped Docker smoke
  supplies separate runtime evidence for the MCP lifecycle.

## CyberOS boundary

The static multi-agent protocol checks pass. The CyberOS-only verifier reports
one protected-WIP finding for the pre-existing tracked
`.github/workflows/ci.yml` change compared with the fixed backup snapshot.
That user WIP was preserved intentionally and was not reverted. This is a
scope/provenance caveat, not evidence that the product source gate failed.

## Current release boundary

Still open:

1. Complete the bounded Sol Ultra re-review of the latest source repair and
   keep its verdict/evidence current after any subsequent source change.
2. Human/operator acceptance of the U0 mobile/desktop UX walkthrough.
3. Raising backend coverage to the plan threshold of 90%.
4. Human lifecycle acceptance and authorization for any commit, push, merge or
   deployment operation.

The product task specs are documented and traceable, but they are not marked
`done` by this run. No commit, push, merge or deployment was performed.
