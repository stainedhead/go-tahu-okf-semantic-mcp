# Tasks: Code & Design Quality Uplift — Auto-Review Fixes

**Feature:** code-and-design-quality-uplift-auto-review
**Created:** 2026-07-06
**Status:** Planning

---

## Progress Summary

0/5 tasks complete

---

## Phase 1: Implementation

### P1.1 — Fix domaintest.VectorStore interface (FR-001)
- **Dependencies:** none
- **Acceptance:** `var _ domain.VectorStore = (*domaintest.VectorStore)(nil)` compiles; `go test -race ./internal/domain/domaintest/... ` passes
- **Status:** ⬜ Not Started

### P1.2 — Migrate a usecase test to domaintest (FR-002)
- **Dependencies:** P1.1
- **Acceptance:** At least one test outside domaintest/ imports domaintest; private duplicate fakes removed from that file; `go test -race ./internal/usecase/...` green
- **Status:** ⬜ Not Started

### P1.3 — Add HNSWStore.Delete test (FR-003)
- **Dependencies:** none (independent)
- **Acceptance:** `TestHNSWStore_Delete` passes; verifies surviving-chunk semantics; vectorstore coverage ≥85%
- **Status:** ⬜ Not Started

### P1.4 — Add CI coverage gates (FR-004)
- **Dependencies:** P1.3 (vectorstore gate needs ≥85% first)
- **Acceptance:** CI gates for adapter/mcp and adapter/vectorstore pass in CI
- **Status:** ⬜ Not Started

### P1.5 — Unexport ValidateConceptPath (FR-005)
- **Dependencies:** none (independent)
- **Acceptance:** `ValidateConceptPath` not exported; `go build ./...` and `golangci-lint` clean
- **Status:** ⬜ Not Started
