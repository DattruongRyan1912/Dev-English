---
id: TASK-PRODUCT-004
title: Complete sourced Knowledge sync and hybrid retrieval
template: task@1
type: feature
module: product
status: on_hold
priority: p0
author: "@codex"
department: engineering
created_at: 2026-08-28T21:33:00+07:00
ai_authorship: generated_then_reviewed
eu_ai_act_risk_class: minimal
client_visible: false
depends_on: [TASK-PRODUCT-003]
new_files:
  - backend/internal/knowledge/
  - backend/internal/connectors/
  - backend/internal/integrations/google_drive.go
  - lib/src/features/knowledge/workspace_knowledge.dart
  - infra/migrations/005_knowledge.sql
  - infra/migrations/010_connector_sync.sql
modified_files:
  - backend/internal/httpapi/v2.go
---

# TASK-PRODUCT-004: Complete sourced Knowledge sync and hybrid retrieval

> Lifecycle hold: wait for `TASK-PRODUCT-003` to complete its human review
> gate before this task can be re-opened and dispatched. This task is not
> eligible for parallel execution.

## Summary

Make manual, Drive and GitHub content searchable as immutable revisions with
source-aware evidence and a degraded full-text fallback.

## Problem

Knowledge is only useful to the assistant when source identity, revision
freshness and evidence links survive synchronization and retrieval.

## Proposed Solution

Keep external systems read-only in V1, persist immutable source revisions and
tombstones, and combine PostgreSQL exact search with the local embedding sidecar
through a workspace-scoped retrieval service.

## Alternatives Considered

- Store only the latest text. Rejected because citations and stale-source detection need revision history.
- Depend only on embeddings. Rejected because exact technical identifiers need lexical search and a degraded path.

## Success Metrics

- Drive and GitHub syncs are idempotent and resume from persisted cursors.
- Deleted external items become searchable tombstones only where policy allows and are removed from active retrieval.
- Search results expose source, revision and evidence locator.

## Scope

In scope: source/revision/chunk/claim/evidence model, Drive bootstrap and
incremental changes, GitHub read sync, hybrid retrieval and Knowledge UI.

Out of scope: Drive write, OCR and arbitrary third-party ingestion.

## Dependencies

- TASK-PRODUCT-003 workspace and work identity.
- PostgreSQL/pgvector migrations and `multilingual-e5-small` sidecar.
- Official Drive authorization and Changes API behavior.

## 1. Description

- Treat every external change as an idempotent revision event within its workspace.
- Preserve evidence IDs and source locators so assistant citations can be verified.
- Fall back to PostgreSQL full-text search and label degraded retrieval when embeddings are unavailable.

## Acceptance criteria

- [ ] AC 1 — Manual sources create immutable revisions, chunks and evidence links. (test: `backend/internal/knowledge/knowledge_test.go::{TestCanonicalClaimRequiresCurrentEvidence; TestEvidenceFreeClaimMustBeNonCanonical; TestRevisionIsAppendOnlyAndFullFieldComparison; TestChunkEmbeddingIsOptionalButFixedDimension}`; `backend/internal/knowledge/postgres_repository_integration_test.go::TestPostgresRepositoryCRUDAndSearch`)
- [ ] AC 2 — Drive bootstrap, incremental changes, removals and cursor replay are covered. (test: `backend/internal/integrations/connectors_read_test.go::TestGoogleDriveReaderBootstrapsPagedFilesBeforeChanges`)
- [ ] AC 3 — Retrieval is workspace-isolated and supports lexical fallback when the sidecar is unavailable. (test: `backend/internal/knowledge/postgres_integration_test.go::TestPostgresKnowledgeWorkspaceAndDeferredEvidenceIntegrity`; `backend/internal/knowledge/postgres_repository_integration_test.go::TestPostgresRepositoryCRUDAndSearch`)

## Edge cases

- The first sync lists existing files before consuming the changes feed.
- A removed file leaves immutable historical revisions but not an active current item.
- Repeated cursor delivery does not duplicate chunks or revisions.

## Protected invariants

- External content is untrusted data and cannot alter system policy.
- Canonical claims require evidence; summaries remain non-canonical.
- Source revisions are immutable after persistence.

## AI Authorship Disclosure

- Tools used: Codex with Go connector, knowledge and PostgreSQL integration tests.
- Scope: re-derived and CONFIRMED: source/revision/evidence ownership; re-derived and CORRECTED: bootstrap and tombstone handling; measured and ADDED: workspace-scoped IDs, cursor and degraded-search evidence.
- Human review: the operator reviews a real Drive/GitHub read-only sync before acceptance.
