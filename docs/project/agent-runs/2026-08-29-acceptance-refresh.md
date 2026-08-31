# Acceptance refresh — 2026-08-29

## Follow-up — current acceptance gate refresh

The local acceptance loop was rerun after the MCP and application boundary
tests were added. The latest backend and frontend gates are green, but the
strict coverage/release boundaries remain explicit.

| Gate | Result |
| --- | --- |
| Go unit suite | exit `0`; `837` passed across `23` packages |
| Go race suite | exit `0`; `837` passed across `23` packages |
| PostgreSQL-enabled coverage | exit `0`; `75.1%` (`8832/11759` statements); Application `90.6%`, Knowledge `90.2%`, MCP `91.2%`, Work `85.5%`, Connectors `80.9%`, HTTP API `83.8%` |
| Go vet/build/diff check | exit `0` |
| Flutter analyze | exit `0`; no issues |
| Flutter full suite | exit `0`; `49` passed, `2` environment skips |
| Flutter Chrome smoke | exit `0`; Today → Work → Knowledge → Learning passed |
| Web release build | exit `0`; `build/web` rebuilt with `--no-wasm-dry-run` |
| Android debug build | exit `0`; `build/app/outputs/flutter-apk/app-debug.apk` generated |
| In-app browser smoke | `http://localhost:8093/` loaded `DevEnglish`; Today rendered, Work/Knowledge/Learning navigation succeeded, `canonical` search returned two results, and captured logs had no error/warning entries |
| Text assistant smoke | Question submitted from Today; transcript remained visible during loading, grounded response returned with an explicit unknown after about `16.4s`, and backend logged the matching `16448ms` request |

The current disposable coverage profile is
`/tmp/devenglish-db-continuation-coverage-3.cov`. The strict `90%` gate for
touched/new packages remains open for HTTP API, Work and connectors; total
coverage is tracked separately against the R0 baseline. Real-device/operator
U0, lifecycle/HITL, independent review and human release authorization also
remain open. No commit, push, merge or deployment was performed.

## Scope

Close the current local acceptance loop after the HTTP/connector coverage
refresh, reproduce the Flutter browser smoke, and record any real regression
instead of treating a successful build as product proof.

## Source change

The first Chrome smoke rendered the application but failed when it navigated to
Learning. The screen used the legacy-metrics fallback in place of the
canonical data-separation statement, so the smoke could not verify the
invariant when legacy endpoints were unavailable.

`lib/src/screens/workspace_learning.dart` now always renders:

> Learning observations stay separate from canonical work data.

Legacy-metrics availability is shown on a separate line. The affected Learning
goldens were regenerated for `390x844`, `430x932` and `1440x900`.

## Evidence

| Gate | Result |
| --- | --- |
| Flutter analyze | exit `0`, no issues |
| Flutter full suite | exit `0`, `49` passed, `2` environment skips |
| Flutter Chrome smoke | exit `0`; Today → Work → Knowledge → Learning passed |
| Web release build | exit `0` with `--no-wasm-dry-run`; `build/web` rebuilt |
| Browser visual smoke | rebuilt Today and Learning rendered; console warnings/errors empty |
| Go unit suite | exit `0`; `649` passed across `23` packages |
| Go race suite | exit `0`; `649` passed across `23` packages |
| PostgreSQL-enabled suite | exit `0`; `663` passed across `23` packages |
| PostgreSQL-enabled coverage | `72.4%` total; application `83.4%`, assistant `92.9%`, knowledge `82.6%`, MCP `84.1%`, work `85.5%`, connectors `80.9%`, HTTP API `77.1%` |
| Android local package | exit `0`; release APK generated at `build/app/outputs/flutter-apk/app-release.apk` and copied to the local desktop handoff path |
| Production-shaped Docker smoke | exit `0`; migration `19`, Work/MCP/Knowledge/auth/CORS/provider-failure/history checks passed |
| Legacy upgrade harness | exit `0`; migrations `000..018`, idempotency, snapshot preservation and workspace isolation passed |
| Knowledge capacity harness | exit `0`; `100000` chunks, FTS lookup and bounded last-page pagination passed |

Additional focused evidence:

- HTTP API focused suite: `89` passed; it covers isolation, query bounds,
  method/`Allow`, error mapping, credential redaction and empty-input
  rejection.
- Connector suite: `32` tests without a database and `34` with PostgreSQL;
  the package reaches `80.9%` in the database-enabled profile.
- Official MCP Go SDK conformance: `62/62` assertions passed, including
  handshake, tool/resource discovery, resource read and bearer-authenticated
  tool call.
- `rtk go vet ./...`, `rtk go build ./...` and `rtk git diff --check`: exit `0`.

## Open acceptance boundary

- The strict plan coverage threshold is `90%`; current total is `72.4%`.
- HTTP API is `77.1%`; it remains the only measured core package below the
  `80%` core-package gate. Older non-core packages also keep the total below
  the strict plan threshold.
- Real-device/operator U0 walkthrough and high-fidelity UX verdict are still
  required.
- Product task lifecycle/HITL acceptance and one fresh independent Sol Ultra
  review are still required; the latest bounded review attempt ended at the
  usage limit without a verdict.
- Human authorization is still required before commit, push, merge or deploy.
- The CyberOS-only verifier reports the protected pre-existing
  `.github/workflows/ci.yml` WIP snapshot finding; that WIP was preserved.

No commit, push, merge or deployment was performed in this run.

## Follow-up — persistence and service boundary test refresh

The continuation added behavior-focused tests for PostgreSQL MCP token
persistence, Knowledge service capabilities and Application composition. The
tests cover invalid replay metadata, conflicting idempotency, expiry/revoke,
corrupt scope rows, unsupported capability/error propagation, blank-search
short-circuiting, invalid identifiers, missing durable pools and safe database
error mapping.

| Gate | Result |
| --- | --- |
| MCP persistence focus | exit `0`; `7` tests passed |
| Knowledge service focus | exit `0`; `135` tests passed |
| Application factory focus | exit `0`; `28` tests passed |
| Go unit suite | exit `0`; `749` passed across `23` packages |
| Go race suite | exit `0`; `749` passed across `23` packages |
| PostgreSQL-enabled suite | exit `0`; `770` passed; `73.4%` total coverage (`8635/11759`); HTTP API `83.8%`, Application `84.3%`, Knowledge `84.1%`, MCP `85.0%` |
| Vet/build/diff/protocol | exit `0`; all four gates passed |
| Production-shaped Docker smoke | exit `0`; `19` migrations and the full Work/Knowledge/MCP/auth/CORS/provider-fail-closed/history flow passed |
| Legacy-upgrade harness | exit `0`; migrations `000..018`, idempotency, snapshot preservation and workspace isolation passed |
| Knowledge capacity gate | exit `0`; `100000` chunks, FTS lookup and bounded last-page pagination passed |

The strict `90%` coverage target, real-device/operator U0, lifecycle/HITL,
independent review and human release authorization remain open. The
CyberOS-only verifier still reports the protected pre-existing
`.github/workflows/ci.yml` WIP snapshot; it was preserved. No commit, push,
merge or deployment was performed.

## Follow-up — HTTP action boundary test refresh

The HTTP action adapter now has focused regression coverage for target
projection, idempotency-key validation, action/MCP error mapping, missing
services, malformed bodies, missing confirmation keys and empty token scopes.
These cases keep malformed or unauthenticated requests at the boundary without
changing the canonical action semantics.

| Gate | Result |
| --- | --- |
| HTTP API package | exit `0`; `159` tests passed; `83.8%` package coverage |
| Go unit suite | exit `0`; `719` passed across `23` packages |
| Go race suite | exit `0`; `719` passed across `23` packages |
| PostgreSQL-enabled suite | exit `0`; `733` passed; `73.1%` total coverage; HTTP API `83.8%` |
| Go vet/build/diff check | exit `0` |

The route-wide workspace-initialization matrix remains `42` passing cases. The
strict `90%` coverage target, real-device/operator U0, lifecycle/HITL,
independent review and human release authorization remain open. The
CyberOS-only verifier still reports the protected pre-existing
`.github/workflows/ci.yml` WIP snapshot; it was preserved. No commit, push,
merge or deployment was performed.

## Follow-up — V2 workspace-scope failure matrix

The HTTP boundary now has a route-wide regression matrix that injects a
workspace-initialization failure and verifies that every V2 route returns the
original server error rather than selecting an implicit workspace.

| Gate | Result |
| --- | --- |
| Scope-failure matrix | exit `0`; `42` V2 route cases passed |
| HTTP API package | exit `0`; `131` tests passed; `82.5%` package coverage |
| Go unit suite | exit `0`; `691` passed across `23` packages |
| Go race suite | exit `0`; `691` passed across `23` packages |
| PostgreSQL-enabled suite | exit `0`; `705` passed; `73.0%` total coverage; HTTP API `82.5%` |
| Go vet/build/diff check | exit `0` |

The strict `90%` coverage target, real-device/operator U0, lifecycle/HITL,
independent review and human release authorization remain open. The
CyberOS-only verifier still reports the protected pre-existing
`.github/workflows/ci.yml` WIP snapshot; it was preserved. No commit, push,
merge or deployment was performed.
