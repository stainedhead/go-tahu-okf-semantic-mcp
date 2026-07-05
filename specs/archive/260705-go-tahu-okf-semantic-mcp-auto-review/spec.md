# Spec: go-tahu-okf-semantic-mcp Auto-Review Fixes

**Feature:** go-tahu-okf-semantic-mcp-auto-review  
**Date:** 2026-07-05  
**Status:** Active — implementing fixes from code review  
**Source PRD:** `specs/260705-go-tahu-okf-semantic-mcp-auto-review/go-tahu-okf-semantic-mcp-auto-review-PRD.md`

---

## Executive Summary

Seven fix items addressing correctness bugs, spec compliance gaps, and a coverage quality-gate shortfall found during the Step 5 code review of `go-tahu-okf-semantic-mcp`. Four P0 blockers must be resolved before merge. All fixes use TDD (Red → Green → Refactor) with a brief code review after each fix before proceeding to the next.

---

## Problem Statement

The `go-tahu-okf-semantic-mcp` implementation passes lint and the race detector, but the code review identified correctness bugs in the OKF repository and use case layers that cause: duplicate log entries per `concept_write`, degraded index quality (missing type/title), data loss under concurrent writes (violates spec AC), and a 13 MB binary committed to the repo. Two additional items address FR-021 compliance (request_id context propagation) and input validation hardening.

**Affected systems:** `internal/adapter/okf/`, `internal/usecase/`, `internal/adapter/mcp/`, `internal/infra/transport/`, `.gitignore`

---

## Goals / Non-Goals

### Goals

- Fix all P0 correctness bugs before merge
- Satisfy the spec quality gate: ≥90% coverage on use case layer
- Close the FR-021 partial miss (request_id in context)
- Add alias format validation to prevent silent MCP access failure

### Non-Goals

- Adding ONNX embedder (deferred to v0.2, ADR-006)
- Extracting `pkg/okfcodec/` (deferred to v0.2, ADR-007)
- Fixing `usecase/bundle.go` filesystem coupling (design smell, not a correctness bug)
- Addressing `SemanticSearch`/`KeywordSearch` behavioral identity (intentional per ADR-006)
- Any feature work beyond the fixes listed below

---

## Functional Requirements

### FR-001: Remove compiled binary from git tracking (maps to FIX-001, P0)

The `tahu` binary at the repo root must be untracked and covered by `.gitignore`. After removal, `go build -o bin/tahu ./cmd/tahu` must continue to work.

### FR-002: Fix double write — remove index/log generation from `FileNodeRepository.Put` (maps to FIX-002, P0)

`FileNodeRepository.Put` must only persist the concept document. All index.md and log.md management must originate exclusively from the use case layer (`ConceptService.WriteConcept`). Every `concept_write` must result in exactly one log entry.

### FR-003: Improve `regenerateIndex` to include frontmatter type and title (maps to FIX-003, P0, depends on FR-002)

`ConceptService.regenerateIndex` must produce an index table with `| File | Type | Title |` columns populated from each concept's frontmatter. Failures to read individual concepts must be tolerated (skip or use empty strings); they must not abort index generation.

### FR-004: Fix concurrent `appendLog` read-modify-write race (maps to FIX-004, P0, depends on FR-002)

The `appendLog` read-then-write sequence must be atomic with respect to concurrent `WriteConcept` calls in the same bundle directory. No log entries may be lost under concurrent writes. `go test -race ./...` must pass on a concurrent write test.

### FR-005: Propagate `request_id` via context (maps to FIX-005, P1)

`loggingMiddleware` must store the generated `request_id` in `context.Context` via `context.WithValue`. An exported `RequestIDFromContext(ctx) string` helper must be provided. Two sequential tool calls must produce distinct IDs.

### FR-006: Validate bundle alias does not contain `:` (maps to FIX-006, P2)

`HandleBundleAdd` must reject aliases containing `:` with a descriptive error message before calling the use case. This prevents silent MCP access failure caused by alias/path misparse in `parseConceptRef`.

### FR-007: Raise test coverage to spec quality gate ≥90% on Use Case layer (maps to FIX-007, P2)

`internal/usecase` must reach ≥90% statement coverage. The following MCP handlers must each have at least one test: `HandleBundleList`, `HandleBundleRemove`, `HandleBundleReindex`, `HandleConceptList`, `HandleConceptLinks`, `HandleIndexRead`, `HandleLogRead`, `HandleConceptTypeList`, `HandleSearchSemantic`, `HandleSearchKeyword`. `domain.ParseScope` must be covered for all scope kinds and the error case.

---

## Non-Functional Requirements

- **Correctness:** Zero data-loss bugs under concurrent access (FR-004 directly addresses the race)
- **Race safety:** `go test -race ./...` must pass after all changes
- **Coverage:** `internal/usecase` ≥ 90% statement coverage (FR-007)
- **Lint:** `golangci-lint run ./...` must remain at 0 issues after each fix
- **Build:** `go build ./...` must succeed after each fix
- **Clean Architecture:** No adapter or infra imports in domain or usecase after any fix

---

## System Architecture — Affected Layers

| Layer | Files Changed | Reason |
|---|---|---|
| Build / gitignore | `.gitignore` | FR-001: cover root-level `tahu` binary |
| Adapter — OKF | `internal/adapter/okf/repository.go` | FR-002: remove GenerateIndex/AppendLog from Put |
| Use Case | `internal/usecase/concept.go` | FR-003/004: improve regenerateIndex, fix appendLog race |
| Adapter — MCP | `internal/adapter/mcp/handlers.go` | FR-006: alias validation in HandleBundleAdd |
| Infra — Transport | `internal/infra/transport/server.go` | FR-005: request_id in context |
| Tests (use case) | `internal/usecase/*_test.go` | FR-002/003/004/007: new and updated tests |
| Tests (MCP adapter) | `internal/adapter/mcp/*_test.go` | FR-006/007: new handler tests |
| Tests (domain) | `internal/domain/*_test.go` | FR-007: ParseScope coverage |

---

## Scope of Changes

**Files to modify:**
- `.gitignore` — add `tahu` pattern (FR-001)
- `internal/adapter/okf/repository.go` — remove `GenerateIndex` + `AppendLog` from `Put` (FR-002)
- `internal/usecase/concept.go` — improve `regenerateIndex`, fix `appendLog` race (FR-003, FR-004)
- `internal/adapter/mcp/handlers.go` — alias colon validation (FR-006)
- `internal/infra/transport/server.go` — `request_id` context propagation (FR-005)

**New test files / additions:**
- `internal/usecase/concept_test.go` — `TestConceptWrite_LogHasSingleEntry`, `TestConceptWrite_IndexContainsTypeAndTitle`, `TestConcurrentConceptWrite_LogPreservesAllEntries`
- `internal/adapter/mcp/handlers_test.go` — handler tests for 9 uncovered handlers + alias validation
- `internal/domain/chunk_test.go` — `ParseScope` coverage

**Files to remove from git tracking:**
- `tahu` (binary, repo root) — `git rm --cached tahu`

**Dependencies introduced:** None — all fixes use existing imports.

---

## Breaking Changes

None. All fixes are additive test coverage or bug fixes with no public API changes. The `RequestIDFromContext` helper in FR-005 is a new exported function — additive, not breaking.

---

## Success Criteria and Acceptance Criteria

### Quality Gates

- `go test -race ./...` passes (all tests including new concurrent test)
- `golangci-lint run ./...` reports 0 issues
- `internal/usecase` coverage ≥ 90% (`go test -coverprofile=coverage.out ./internal/usecase/...`)
- `git ls-files tahu` returns empty output
- No adapter or infra imports in domain or usecase packages

### Per-FR Acceptance Criteria

**FR-001:**
1. `git ls-files tahu` returns empty
2. `.gitignore` covers `tahu`
3. `go build -o bin/tahu ./cmd/tahu` succeeds

**FR-002:**
1. `TestConceptWrite_LogHasSingleEntry` fails before fix, passes after
2. `FileNodeRepository.Put` contains no `GenerateIndex` or `AppendLog` calls
3. All existing `TestWriteConcept_*` tests pass

**FR-003:**
1. `TestConceptWrite_IndexContainsTypeAndTitle` fails before fix, passes after
2. Index table format is `| File | Type | Title |` with values from frontmatter

**FR-004:**
1. `TestConcurrentConceptWrite_LogPreservesAllEntries` exposes race or fails before fix
2. `go test -race ./...` clean after fix
3. All existing tests pass

**FR-005:**
1. `TestLoggingMiddleware_RequestIDInContext` fails before fix, passes after
2. `RequestIDFromContext` exported from transport package
3. Two sequential calls produce distinct IDs

**FR-006:**
1. `TestHandleBundleAdd_RejectsColonInAlias` fails before fix, passes after
2. Error message references the `:` constraint

**FR-007:**
1. `internal/usecase` reports ≥ 90% coverage
2. 9 previously-uncovered handlers each have one happy-path test
3. `ParseScope` covered for all scope kinds and error case

---

## Risks and Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| FR-003's N+1 `Get` calls per `regenerateIndex` slow writes on large directories | Low (v0.1 bundles are small) | Low | Accept for v0.1; optimize with batch fetch in v0.2 if needed |
| FR-004's mutex approach affects `WriteConcept` throughput under concurrent load | Low | Low | Bundle-scoped mutex; concurrent writes to different bundles are unaffected |
| Removing `GenerateIndex` from `Put` breaks a test that uses real `FileNodeRepository` | Possible | Low | Run tests after FR-002 before proceeding; catch immediately |

---

## Timeline and Milestones

All fixes are independent of each other except: FR-003 and FR-004 depend on FR-002 completing first. FR-001, FR-005, FR-006 are fully independent.

| Milestone | FIX IDs | Can parallelize? |
|---|---|---|
| M1: Binary cleanup | FR-001 | Standalone |
| M2: Core write correctness | FR-002 → FR-003, FR-004 | FR-003 and FR-004 run after FR-002 |
| M3: Transport compliance | FR-005 | Standalone |
| M4: Validation hardening | FR-006 | Standalone |
| M5: Coverage gate | FR-007 | After all other fixes (to include their new tests in count) |

---

## References

- Source PRD: `specs/260705-go-tahu-okf-semantic-mcp-auto-review/go-tahu-okf-semantic-mcp-auto-review-PRD.md`
- Original spec (archived): `specs/archive/260705-go-tahu-okf-semantic-mcp/spec.md`
- Architecture decisions: `documents/arch-decisions-record.md` (ADR-001 through ADR-007)
- Parent dev-flow dashboard: `DEV-FLOW-STATUS.md`
