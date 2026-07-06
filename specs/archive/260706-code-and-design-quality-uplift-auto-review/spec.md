# Spec: Code & Design Quality Uplift — Auto-Review Fixes

**Feature:** code-and-design-quality-uplift-auto-review
**Created:** 2026-07-06
**Status:** Draft
**Source PRD:** specs/260706-code-and-design-quality-uplift-auto-review/code-and-design-quality-uplift-auto-review-PRD.md

---

## Executive Summary

The Step 5 automated code review of the `feat/code-and-design-quality-uplift` branch found two blocking defects and three unmet commitments. This spec drives the resolution: fix the `domaintest.VectorStore` broken interface contract, complete the FR-032 domaintest migration, add a missing test for `HNSWStore.Delete`, add two missing CI coverage gates, and remove the orphaned `ValidateConceptPath` export.

---

## Problem Statement

Two Must Fix items and three Warnings were found in code review:

1. **Must Fix — domaintest.VectorStore interface mismatch**: The fake has `DeleteByBundle`/`DeleteByIDs` but the `domain.VectorStore` interface requires `Delete(ctx, ids []string) error`. Fails compile when used as the interface type.
2. **Must Fix — domaintest unused (FR-032 incomplete)**: The package was created but never imported. Old fakes in `fake_test.go` and `search_test.go` still exist. The goal of a shared, importable fake layer is unmet.
3. **Warning — HNSWStore.Delete at 0% coverage**: The stale-vector delete path (FR-010) has no tests. Critical correctness cannot be verified.
4. **Warning — Two CI coverage gates missing**: `adapter/mcp` (spec: ≥75%, actual: 61.4%) and `adapter/vectorstore` (spec: ≥85%, actual: 76.4%) have no CI enforcement.
5. **Warning — ValidateConceptPath orphaned export**: No longer called in production (all routing goes through BundlePathResolver). Contradicts FR-005.

---

## Goals

1. Fix the `domaintest.VectorStore` interface mismatch so it satisfies `domain.VectorStore`
2. Complete FR-032 by importing domaintest from at least one real test consumer and removing duplicate fakes
3. Add a test for `HNSWStore.Delete` verifying the stale-vector delete path
4. Add CI gates enforcing adapter/mcp ≥75% and adapter/vectorstore ≥85%
5. Remove the orphaned `ValidateConceptPath` export

---

## Non-Goals

- New features or domain changes beyond fixing review findings
- Raising coverage floors beyond spec-defined targets
- Replacing any existing correct implementation
- Windows platform support

---

## Functional Requirements

**FR-001:** `domaintest.VectorStore` must implement `domain.VectorStore` completely. Replace `DeleteByBundle`/`DeleteByIDs` with `Delete(ctx context.Context, ids []string) error`. Add compile-time assertion `var _ domain.VectorStore = (*VectorStore)(nil)`.

**FR-002:** The domaintest package must be imported by at least one test file outside `internal/domain/domaintest/`. Migrate `internal/usecase/bundle_test.go` or `concept_test.go` to use `domaintest.BundleRepository` and `domaintest.NodeRepository`. Remove or reduce now-redundant private fakes from those files.

**FR-003:** Add `TestHNSWStore_Delete` in `internal/adapter/vectorstore/hnsw_test.go`: upsert two chunks, delete one by ID, verify Search returns only the survivor, verify deleted chunk is absent from store state.

**FR-004:** Add CI coverage gates for `internal/adapter/mcp` (≥65% as step-wise ratchet — scope the gap first; raise to ≥75% if achievable without excessive test authoring) and `internal/adapter/vectorstore` (≥85%) in `.github/workflows/ci.yml`.

**FR-005:** Unexport or delete `ValidateConceptPath` from `internal/adapter/okf/validator.go`. Update `repository_coverage_test.go` to exercise `BundlePathResolver.Resolve` instead if the function is deleted.

---

## Non-Functional Requirements

- **TDD:** Every fix RED→GREEN→REFACTOR. No production change without a failing test first.
- **Code review per fix:** Brief design/correctness/security review before advancing to next FR.
- **Race detector:** `go test -race ./...` green after every task.
- **Lint:** `golangci-lint run ./...` 0 issues after every task.
- **Parallel workstreams:** FR-003, FR-004, FR-005 are independent; FR-001 must land before FR-002.
- **Agent teammates:** Use worker agents for parallel tasks; all worker agents use `claude-sonnet-4-6`.

---

## System Architecture

### Affected layers

| Layer | Impact |
|---|---|
| `internal/domain/domaintest/` | FR-001, FR-002: fix Delete method, add compile assertion |
| `internal/usecase/bundle_test.go` or `concept_test.go` | FR-002: import domaintest, remove private duplicates |
| `internal/adapter/vectorstore/hnsw_test.go` | FR-003: add Delete test |
| `.github/workflows/ci.yml` | FR-004: add two coverage gate steps |
| `internal/adapter/okf/validator.go` | FR-005: unexport ValidateConceptPath |
| `internal/adapter/okf/repository_coverage_test.go` | FR-005: update if ValidateConceptPath deleted |

### No new components required

All changes are corrections to existing files.

---

## Scope of Changes

### Files to modify
- `internal/domain/domaintest/fakes.go` — FR-001, FR-002
- `internal/usecase/bundle_test.go` or `internal/usecase/concept_test.go` — FR-002
- `internal/adapter/vectorstore/hnsw_test.go` — FR-003
- `.github/workflows/ci.yml` — FR-004
- `internal/adapter/okf/validator.go` — FR-005
- `internal/adapter/okf/repository_coverage_test.go` — FR-005 (if delete chosen)

### Breaking changes
- None. `ValidateConceptPath` is only called from tests; unexport is test-internal only.

---

## Acceptance Criteria

- [ ] `var _ domain.VectorStore = (*domaintest.VectorStore)(nil)` compiles without error
- [ ] At least one test file outside `internal/domain/domaintest/` imports the domaintest package
- [ ] `go test ./internal/adapter/vectorstore/... -run TestHNSWStore_Delete` passes; verifies surviving-chunk semantics
- [ ] CI adapter/mcp gate passes at the chosen threshold; adapter/vectorstore gate passes at ≥85%
- [ ] `ValidateConceptPath` is no longer exported from `internal/adapter/okf`
- [ ] `go test -race ./...` green
- [ ] `golangci-lint run ./...` 0 issues
- [ ] All existing tests still pass

---

## Risks and Mitigation

| Risk | Mitigation |
|---|---|
| adapter/mcp gap (61.4%→75%) too large | Scope gap first; ratchet to ≥65% if needed |
| FR-002 migration breaks existing tests | Run full suite after each migration step |
| ValidateConceptPath removal misses a caller | `grep -rn ValidateConceptPath` before deleting |

---

## References

- Source PRD: `specs/260706-code-and-design-quality-uplift-auto-review/code-and-design-quality-uplift-auto-review-PRD.md`
- Original spec (archived): `specs/archive/260705-code-and-design-quality-uplift/`
- Code review findings: Step 5 of dev-flow run started 2026-07-05
