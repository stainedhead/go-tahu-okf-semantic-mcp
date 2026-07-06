# PRD: Code & Design Quality Uplift — Auto-Review Fixes

**Created:** 2026-07-06
**Jira:** N/A
**Status:** Draft
**Branch:** feat/code-and-design-quality-uplift
**Source review:** Step 5 automated code review of the quality-uplift branch

---

## Problem Statement

The automated code review of the `feat/code-and-design-quality-uplift` branch found two blocking defects and three unmet coverage commitments. The most critical is a broken interface contract in the `domaintest` package that causes a compile failure the moment any test tries to use the package as intended. A secondary Must Fix is that FR-032 ("promote fakes to domaintest/") is structurally incomplete: the package was created but never imported, and the old duplicated fakes were not removed. Three warnings round out the findings: `HNSWStore.Delete` has 0% test coverage despite being the core of the FR-010 stale-vector fix; two adapter-layer coverage floors declared in the spec are not enforced by CI; and `ValidateConceptPath` is an orphaned export that contradicts FR-005's consolidation goal.

---

## Goals

1. Fix the `domaintest.VectorStore` interface mismatch so the package compiles correctly when used as `domain.VectorStore`
2. Complete FR-032 by wiring domaintest into at least one real consumer and removing the redundant old fakes
3. Add test coverage for `HNSWStore.Delete` so the FR-010 stale-vector fix is verified
4. Add CI coverage gates for `adapter/mcp` (≥75%) and `adapter/vectorstore` (≥85%)
5. Remove the orphaned `ValidateConceptPath` export (or unexport it) to satisfy FR-005

---

## Non-Goals

- New features or additional domain changes beyond fixing the review findings
- Raising coverage floors beyond the spec-defined targets
- Replacing any existing correct implementation

---

## Functional Requirements

**FR-R01 (P0 — Blocker):** `domaintest.VectorStore` must implement `domain.VectorStore` interface completely. Replace `DeleteByBundle` and `DeleteByIDs` with `Delete(ctx context.Context, ids []string) error`. Add a compile-time assertion `var _ domain.VectorStore = (*VectorStore)(nil)`.

**FR-R02 (P0 — Blocker):** The `domaintest` package must be imported by at least one test outside `internal/domain/domaintest/`. Migrate `internal/usecase/bundle_test.go` or `concept_test.go` to use `domaintest.BundleRepository` and `domaintest.NodeRepository` instead of their private fakes. Remove or reduce the now-redundant private fakes.

**FR-R03 (P1 — High):** Add a test for `HNSWStore.Delete`: upsert two chunks, delete one by ID, verify that `Search` returns only the surviving chunk and the deleted chunk is absent.

**FR-R04 (P1 — High):** Add CI coverage gates for `internal/adapter/mcp` (≥75%) and `internal/adapter/vectorstore` (≥85%) in `.github/workflows/ci.yml`, matching the pattern of existing gates. Both packages must meet their floor before the gate passes.

**FR-R05 (P2 — Medium):** `ValidateConceptPath` in `internal/adapter/okf/validator.go` must be unexported (rename to `validateConceptPath`) or deleted. Update `repository_coverage_test.go` to exercise the `BundlePathResolver.Resolve` path instead of the old standalone function if deletion is chosen.

---

## Non-Functional Requirements

- **TDD:** Every fix must be driven RED→GREEN→REFACTOR. No production change without a failing test first.
- **Race detector:** `go test -race ./...` must remain green after every task.
- **Lint:** `golangci-lint run ./...` must return 0 issues after every task.
- **Parallel workstreams:** FR-R03, FR-R04, and FR-R05 are independent and may be implemented in parallel git worktrees.

---

## Acceptance Criteria

- [ ] `var _ domain.VectorStore = (*domaintest.VectorStore)(nil)` compiles without error
- [ ] At least one test file outside `internal/domain/domaintest/` imports and uses the domaintest package
- [ ] `go test ./internal/adapter/vectorstore/... -run TestHNSWStore_Delete` passes and verifies surviving-chunk semantics
- [ ] CI `adapter/mcp` gate enforces ≥75%; CI `adapter/vectorstore` gate enforces ≥85%
- [ ] `ValidateConceptPath` is no longer exported from `internal/adapter/okf`
- [ ] `go test -race ./...` green
- [ ] `golangci-lint run ./...` 0 issues
- [ ] All existing tests still pass

---

## Dependencies and Risks

| Item | Type | Notes |
|------|------|-------|
| FR-R02 requires FR-R01 | Dependency | Can't migrate consumers until interface is fixed |
| Raising adapter/mcp to ≥75% | Risk | May require significant new test authoring; scope the gap before committing |
| Removing ValidateConceptPath | Risk | Only test consumer is `repository_coverage_test.go` — update that test simultaneously |

---

## Open Questions

- None. All decisions are made in the spec above.
