# Workspace resolution guard — 2026-08-29

## Purpose

Close the legacy workspace-resolution gap without inventing a production
mapping policy. The application still derives the V1 deterministic workspace
ID from the authenticated user, but PostgreSQL must now refuse ambiguous or
unsafe topology instead of silently selecting a scope.

## Implemented scope

- `PostgresWorkspaceRepository.EnsureWorkspace` now validates its context and
  identifiers before database access;
- the authenticated owner row is locked in a transaction, serializing lazy
  resolution for one user;
- a fresh user with no workspace history may receive the deterministic
  workspace exactly once, and repeat/concurrent resolution is idempotent;
- an existing active target is accepted only when it is the user's sole
  active workspace;
- owner mismatch and deleted target rows fail with
  `ErrWorkspaceUnavailable`;
- a missing target with any active or deleted workspace history, and multiple
  active workspaces, fail with `ErrWorkspaceMappingRequired`;
- missing owner rows fail with `ErrWorkspaceOwnerUnavailable`;
- the HTTP product error boundary exposes these mapping conditions as `503`
  instead of pretending the request has a usable workspace;
- no migration, legacy backfill, workspace switcher or production mapping
  policy was changed.

## Evidence

| Check | Observed result |
| --- | --- |
| Targeted default application/HTTP suite | exit `0`; `292` tests passed in 2 packages |
| Targeted PostgreSQL application/HTTP suite | exit `0`; `301` tests passed in 2 packages |
| Full default Go normal/race | exit `0`; `985` tests passed across 23 packages for each run |
| Full PostgreSQL Go normal/race | exit `0`; `1060` tests passed across 23 packages for each run |
| PostgreSQL raw coverage | exit `0`; `9354/12085 = 77.40%`; all seven required packages passed `>=90%` |
| `go vet ./...` | exit `0`; no issues found |
| `go build ./...` | exit `0` |
| `git diff --check` | exit `0` |
| Production-shaped Docker smoke | exit `0`; 19 migrations and auth/Work/Knowledge/MCP/CORS/provider-failure checks passed |
| CyberOS machine gates | exit `0`; build/lint/test/coverage/status generation GREEN; doctor skipped because memory CLI is unavailable |
| Task lint and multi-agent protocol verifier | exit `0`; TRACE-001 remains informational only; protocol/entrypoints/manifests PASS |

Coverage profile:
`/tmp/devenglish-go-db-20260829-workspace-resolution2.cov`

Package raw coverage from the gate:

- Application `752/830 = 90.60%`;
- Assistant `274/295 = 92.88%`;
- Connectors `1133/1258 = 90.06%`;
- HTTP API `1169/1297 = 90.13%`;
- Knowledge `833/923 = 90.25%`;
- MCP `1358/1495 = 90.84%`;
- Work `1581/1756 = 90.03%`.

## Security and data-integrity boundary

The guard prevents an authenticated request from silently creating a second
workspace when legacy workspace history exists. It also prevents a workspace
ID owned by another user or marked deleted from becoming the active scope.
The tests use unique synthetic users and do not call live providers or expose
credentials.

## Remaining decision

An operator/product owner must still define the authoritative mapping for
legacy users with zero, multiple, deleted or restored workspaces before a
production backfill or resolver can be enabled. This evidence does not claim
that decision, independent Sol review, CI on a delivery commit, task
lifecycle acceptance or Git/release authorization.

## Delivery boundary

Changes remain in the existing dirty development checkout. No file was
staged, committed, pushed, merged or deployed by this slice.
