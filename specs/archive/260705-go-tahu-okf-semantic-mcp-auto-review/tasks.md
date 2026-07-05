# Tasks: go-tahu-okf-semantic-mcp-auto-review

**Feature:** go-tahu-okf-semantic-mcp-auto-review  
**Date:** 2026-07-05  
**Status:** Planning

---

## Progress Summary

0 / 13 tasks complete

---

## Phase 1: P0 Fixes

### P1.1 — FIX-001: Remove tahu binary from git tracking
**Dependencies:** None  
**Estimate:** 5 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `git rm --cached tahu` executed
- [ ] `tahu` added to `.gitignore`
- [ ] `git ls-files tahu` returns empty
- [ ] `go build -o bin/tahu ./cmd/tahu` succeeds

---

### P1.2 — FIX-002: Write failing test `TestConceptWrite_LogHasSingleEntry` (RED)
**Dependencies:** None  
**Estimate:** 15 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] Test written in `internal/usecase/concept_test.go` or a new integration test using real `FileNodeRepository`
- [ ] Test fails with "got 2 log entries, want 1" (or similar)

---

### P1.3 — FIX-002: Remove `GenerateIndex`/`AppendLog` from `FileNodeRepository.Put` (GREEN)
**Dependencies:** P1.2  
**Estimate:** 20 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `FileNodeRepository.Put` contains no call to `GenerateIndex` or `AppendLog`
- [ ] `TestConceptWrite_LogHasSingleEntry` passes
- [ ] All existing `TestWriteConcept_*` tests pass
- [ ] Brief code review completed before moving to P1.4

---

### P1.4 — FIX-003: Write failing test `TestConceptWrite_IndexContainsTypeAndTitle` (RED)
**Dependencies:** P1.3  
**Estimate:** 15 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] Test written; asserts index.md contains type and title values
- [ ] Test fails before fix

---

### P1.5 — FIX-003: Improve `regenerateIndex` with type+title (GREEN)
**Dependencies:** P1.4  
**Estimate:** 25 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `regenerateIndex` calls `NodeRepository.Get` per ref
- [ ] Index table format: `| File | Type | Title |`
- [ ] Get failures tolerated (empty values, no abort)
- [ ] `TestConceptWrite_IndexContainsTypeAndTitle` passes
- [ ] Brief code review before P1.6

---

### P1.6 — FIX-004: Write failing/racy test `TestConcurrentConceptWrite_LogPreservesAllEntries` (RED)
**Dependencies:** P1.3  
**Estimate:** 20 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] Test spins up N goroutines writing to same directory
- [ ] Test fails or detects race with `go test -race`

---

### P1.7 — FIX-004: Add bundle-scoped advisory mutex to `ConceptService` (GREEN)
**Dependencies:** P1.6  
**Estimate:** 30 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `ConceptService` has `bundleMu sync.Map` field
- [ ] `WriteConcept` wraps Put + regenerateIndex + appendLog in per-bundle mutex
- [ ] `TestConcurrentConceptWrite_LogPreservesAllEntries` passes
- [ ] `go test -race ./...` reports no data races
- [ ] Brief code review before Phase 2

---

## Phase 2: P1 Fix

### P2.1 — FIX-005: Write failing test `TestLoggingMiddleware_RequestIDInContext` (RED)
**Dependencies:** None (independent)  
**Estimate:** 15 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] Test wraps a fake handler that reads request_id from context
- [ ] Test fails before fix

---

### P2.2 — FIX-005: Add context propagation to `loggingMiddleware` (GREEN)
**Dependencies:** P2.1  
**Estimate:** 20 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `loggingMiddleware` calls `context.WithValue`
- [ ] `RequestIDFromContext` exported from transport package
- [ ] `TestLoggingMiddleware_RequestIDInContext` passes
- [ ] Brief code review

---

## Phase 3: P2 Fixes

### P3.1 — FIX-006: Write failing test `TestHandleBundleAdd_RejectsColonInAlias` (RED)
**Dependencies:** None (independent)  
**Estimate:** 10 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] Test calls `bundle_add` with `alias = "bad:alias"`
- [ ] Test fails before fix

---

### P3.2 — FIX-006: Add alias colon validation to `HandleBundleAdd` (GREEN)
**Dependencies:** P3.1  
**Estimate:** 10 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `strings.Contains(alias, ":")` check added
- [ ] Error message references `:` constraint
- [ ] `TestHandleBundleAdd_RejectsColonInAlias` passes

---

### P3.3 — FIX-007: Write missing handler tests for 9 uncovered handlers
**Dependencies:** P1.x, P2.x, P3.1–P3.2 (count new tests toward coverage)  
**Estimate:** 60 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] Happy-path test for each: HandleBundleList, HandleBundleRemove, HandleBundleReindex, HandleConceptList, HandleConceptLinks, HandleIndexRead, HandleLogRead, HandleConceptTypeList, HandleSearchSemantic, HandleSearchKeyword
- [ ] Tests are table-driven where appropriate
- [ ] Test names reference spec section (e.g., `_SpecFR012`)

---

### P3.4 — FIX-007: Reach ≥90% usecase coverage + ParseScope coverage
**Dependencies:** P3.3  
**Estimate:** 30 min  
**Status:** ⬜ Pending

Acceptance:
- [ ] `ParseScope` tested for: `global`, `bundle:x`, `path:x:y`, invalid input
- [ ] `go test -coverprofile=coverage.out ./internal/usecase/...` reports ≥ 90%
- [ ] All tests pass with `go test -race ./...`
- [ ] `golangci-lint run ./...` reports 0 issues
