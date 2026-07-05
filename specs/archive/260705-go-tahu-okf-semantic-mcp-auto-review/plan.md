# Plan: go-tahu-okf-semantic-mcp-auto-review

**Feature:** go-tahu-okf-semantic-mcp-auto-review  
**Date:** 2026-07-05  
**Status:** Planning

---

## Development Approach

TDD (Red → Green → Refactor) for every fix. Write the failing test first. Then write the minimum production code to make it pass. Then refactor. Conduct a brief code and design review after each fix before moving to the next. Commit after each fix with a message referencing the FIX ID.

---

## Phase Breakdown

### Phase 1: P0 Fixes (blockers)

**1a — FIX-001 (independent):** Remove binary from git tracking
- `git rm --cached tahu`
- Add `tahu` to `.gitignore`
- Verify `go build -o bin/tahu ./cmd/tahu` works

**1b — FIX-002 (prerequisite for 1c, 1d):** Remove index/log from Put
- Write `TestConceptWrite_LogHasSingleEntry` (RED)
- Remove `GenerateIndex` + `AppendLog` from `FileNodeRepository.Put`
- GREEN + Refactor
- Brief review

**1c — FIX-003 (after 1b):** Improve regenerateIndex
- Write `TestConceptWrite_IndexContainsTypeAndTitle` (RED)
- Update `regenerateIndex` to call Get per ref and build `| File | Type | Title |`
- GREEN + Refactor
- Brief review

**1d — FIX-004 (after 1b):** Fix concurrent appendLog race
- Write `TestConcurrentConceptWrite_LogPreservesAllEntries` (RED or race-detect-fail)
- Add bundle-scoped advisory mutex to `ConceptService`
- Wrap Put + regenerateIndex + appendLog in the mutex
- GREEN + `go test -race ./...` clean
- Brief review

### Phase 2: P1 Fix

**2a — FIX-005 (independent):** request_id context propagation
- Write `TestLoggingMiddleware_RequestIDInContext` (RED)
- Update `loggingMiddleware` to `context.WithValue`
- Add `RequestIDFromContext` helper
- GREEN + Refactor
- Brief review

### Phase 3: P2 Fixes

**3a — FIX-006 (independent):** Alias colon validation
- Write `TestHandleBundleAdd_RejectsColonInAlias` (RED)
- Add `strings.Contains(alias, ":")` check in `HandleBundleAdd`
- GREEN + Refactor

**3b — FIX-007 (after all other fixes):** Coverage gate
- Run `go test -coverprofile=coverage.out ./internal/usecase/...` and identify gaps
- Write table-driven handler tests for 9 uncovered handlers
- Write `ParseScope` tests
- Reach ≥90% on usecase package

---

## Critical Path

FIX-001, FIX-005, FIX-006 are independent. FIX-002 → FIX-003 → FIX-007. FIX-002 → FIX-004.

```
FIX-001 (parallel)
FIX-002 → FIX-003 \
          FIX-004  → FIX-007
FIX-005 (parallel)
FIX-006 (parallel)
```

---

## Testing Strategy

- Each FIX gets its own failing test written first
- Concurrent test uses goroutines + `sync.WaitGroup`; validated with `go test -race`
- Handler tests are table-driven in `internal/adapter/mcp/handlers_test.go`
- Coverage measured with `go test -coverprofile=coverage.out ./internal/usecase/...`

---

## Rollout Strategy

All fixes are on branch `feat/go-tahu-okf-semantic-mcp`. No staged rollout needed — these are internal correctness fixes with no user-visible API changes. Merge when all quality gates pass.

---

## Success Metrics

- `git ls-files tahu` returns empty
- `go test -race ./...` passes
- `internal/usecase` coverage ≥ 90%
- `golangci-lint run ./...` reports 0 issues
- `go build ./...` succeeds
