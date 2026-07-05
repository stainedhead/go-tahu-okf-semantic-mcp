# Auto-Review PRD: go-tahu-okf-semantic-mcp

**Source branch:** `feat/go-tahu-okf-semantic-mcp`  
**Review date:** 2026-07-05  
**Reviewer:** Claude Code (automated code review, Step 5 of implm-frm-prd)  
**Review PRD file:** `go-tahu-okf-semantic-mcp-auto-review-PRD.md`

---

## Executive Summary

The implementation is structurally sound: Clean Architecture dependency rule holds throughout, all 14 MCP tools are implemented with real handlers (not stubs), lint is clean, and the race detector passes. Security validation (path traversal, size caps) is thorough and layered.

Four correctness bugs require P0 fixes before merge: a committed 13 MB binary bloats the repo; `FileNodeRepository.Put` and `ConceptService.WriteConcept` both independently regenerate `index.md` and `log.md` causing duplicate log entries and a degraded index; and the `appendLog` read-modify-write is outside the write mutex, enabling data-loss races that violate the spec's concurrent-write guarantee.

One P1 item addresses a spec compliance gap (FR-021 request_id context propagation). Two P2 items address validation hardening and the spec quality-gate coverage shortfall.

**Overall verdict: Approve with fixes — do not merge until P0 items are resolved.**

Implementation note: All fixes must use TDD (Red → Green → Refactor). Write a failing test for each fix before writing any production code. Conduct a brief code and design review after each fix is completed before moving to the next.

---

## Functional Requirements

---

### FIX-001: Remove compiled binary from git tracking (P0 — blocker)

**Priority:** P0  
**Severity:** Build hygiene / security

**Problem:** The compiled binary `tahu` (13 MB) is committed to the repository root. The `.gitignore` covers `bin/`, `dist/`, and OS-specific extensions (`*.exe`, `*.so`, `*.dll`, `*.dylib`) but not a plain `tahu` binary at the root. Every clone inflates by 13 MB and ships a platform-specific macOS arm64 binary to all contributors.

**Acceptance Criteria:**

1. `git ls-files tahu` returns empty output — the binary is not tracked.
2. `echo "tahu" | git check-ignore -v --stdin` confirms `tahu` is covered by `.gitignore`.
3. `go build -o bin/tahu ./cmd/tahu` succeeds and places the output under `bin/`.
4. `bin/` is listed in `.gitignore` (already present) so the build output is never committed.
5. `git log --all --oneline -- tahu` shows the removal commit as the most recent change to that path.

---

### FIX-002: Fix double write — remove index/log generation from `FileNodeRepository.Put` (P0 — blocker)

**Priority:** P0  
**Severity:** Correctness — duplicate log entries, degraded index quality

**Problem:** Two layers both regenerate `index.md` and `log.md` on every `concept_write`:

- `FileNodeRepository.Put` (adapter): calls `GenerateIndex` (good quality: type+title) and `AppendLog` (one log entry).
- `ConceptService.WriteConcept` (use case): then calls `regenerateIndex` (lower quality: filename only, empty Type column) and `appendLog` (second log entry).

Net result per write: `log.md` accumulates two entries; `index.md` is overwritten twice with the lower-quality version winning. The tests miss this because `FakeNodeRepository.Put` has no index/log logic.

Per Clean Architecture: index and log management is the use case's responsibility. The adapter's `Put` should only persist the concept document.

**Acceptance Criteria:**

1. A new test `TestConceptWrite_LogHasSingleEntry` registers a bundle, calls `WriteConcept` once, reads `log.md`, and asserts exactly one log entry is present. This test must fail before the fix.
2. `FileNodeRepository.Put` does not call `GenerateIndex` or `AppendLog`. All index and log writes originate in the use case layer only.
3. `TestConceptWrite_LogHasSingleEntry` passes after the fix.
4. Existing tests for `ConceptService.WriteConcept` (including `TestWriteConcept_*`) continue to pass.
5. All tests pass with `go test -race ./...`.
6. No new Clean Architecture violations (adapter/infra imports in domain/usecase).

---

### FIX-003: Improve `regenerateIndex` to include frontmatter type and title (P0 — blocker)

**Priority:** P0  
**Severity:** Correctness — spec requires index to show type; current implementation always emits empty Type column

**Problem:** `ConceptService.regenerateIndex` only has `ConceptRef` values and generates `| Path | Type |` with an empty Type column. After FIX-002 removes the adapter-side `GenerateIndex` call, this becomes the sole index writer — and it generates a degraded index that does not reflect each concept's frontmatter type or title.

The spec FR-011 requires `index.md` to be regenerated on success; the OKF convention is that `index.md` serves as navigation and must reflect the typed structure of the directory.

**Acceptance Criteria:**

1. A new test `TestConceptWrite_IndexContainsTypeAndTitle` writes two concepts with distinct types, calls `WriteConcept`, reads `index.md`, and asserts both the type and title values appear in the index table. This test must fail before the fix.
2. `regenerateIndex` calls `NodeRepository.Get` per ref to retrieve `Frontmatter.Type` and `Frontmatter.Title`. Get failures for individual refs are tolerated (skip that row or use empty strings); they do not abort index generation.
3. The index table format is `| File | Type | Title |` with `|---|---|---|` separator row, consistent with the adapter's `GenerateIndex` output.
4. `TestConceptWrite_IndexContainsTypeAndTitle` passes after the fix.
5. All existing tests pass with `go test -race ./...`.

---

### FIX-004: Fix concurrent `appendLog` read-modify-write race (P0 — blocker)

**Priority:** P0  
**Severity:** Correctness — log entries silently lost under concurrent writes; spec AC explicitly requires no data corruption on concurrent writes  
**Depends on:** FIX-002 (mutex boundary changes with the removal of index/log from Put)

**Problem:** `ConceptService.appendLog` performs ReadReserved → string concat → WriteReserved. Neither `ReadReserved` nor `WriteReserved` holds the `FileNodeRepository` write mutex. Two concurrent `WriteConcept` calls for concepts in the same directory can interleave their read-modify-write sequences, causing one log entry to be silently lost.

The spec edge case says: "Concurrent `concept_write` to same path — both complete without data corruption. Bundle-level write mutex ensures serialization." The mutex currently only covers `FileNodeRepository.Put`, not the subsequent `appendLog`.

**Acceptance Criteria:**

1. A new test `TestConcurrentConceptWrite_LogPreservesAllEntries` concurrently calls `WriteConcept` on two different concepts in the same directory from N goroutines, then reads `log.md` and asserts it contains exactly N log entries. This test must either fail (data race detected) or be racy before the fix.
2. The fix ensures that the read-then-write of `log.md` is atomic with respect to other writes in the same bundle. Acceptable approaches: a bundle-scoped mutex held across the full WriteConcept flow; or an append-only `WriteReserved` variant; or moving log management back into a single locked adapter operation. Do not reintroduce the double-write fixed in FIX-002.
3. `go test -race ./...` reports no data race on the concurrent test.
4. All existing tests pass.

---

### FIX-005: Propagate `request_id` via context (FR-021 compliance) (P1 — high)

**Priority:** P1  
**Severity:** Spec compliance — FR-021 requires propagation via context  
**Dependency:** Requires Go 1.21+ (already in use; `context.WithoutCancel` in `server.go` confirms the version constraint is met)

**Problem:** `loggingMiddleware` generates `requestID := uuid.New().String()` and logs it, but never stores it in `context.Context`. FR-021 states the `request_id` must be "propagated via context" so downstream handlers and use cases can correlate their own log lines to the originating MCP call.

**Acceptance Criteria:**

1. A new test `TestLoggingMiddleware_RequestIDInContext` wraps a fake handler that reads `request_id` from context and asserts it is non-empty. This test must fail before the fix.
2. `loggingMiddleware` calls `context.WithValue(ctx, <unexported key>, requestID)` before calling `next`.
3. An exported helper function `RequestIDFromContext(ctx context.Context) string` is added to the `transport` package, returning the stored request_id or `""` if absent.
4. Two sequential tool calls produce distinct `request_id` values (FR-021 AC).
5. `TestLoggingMiddleware_RequestIDInContext` passes after the fix.
6. All existing tests pass.

---

### FIX-006: Validate bundle alias does not contain `:` (P2 — medium)

**Priority:** P2  
**Severity:** Correctness — alias containing `:` silently breaks all MCP access to the bundle

**Problem:** `parseConceptRef` splits the concept ref string on the first `:` to extract the bundle alias. An alias registered with a `:` character (e.g. `"my:kb"`) makes all its concepts unreachable via MCP tools — the ref `"my:kb:notes.md"` is misinterpreted as alias `"my"`, path `"kb:notes.md"`, then fails with a misleading `ErrNotFound`. The error gives no indication that the alias itself is malformed.

**Acceptance Criteria:**

1. A new test `TestHandleBundleAdd_RejectsColonInAlias` calls `bundle_add` with `alias = "bad:alias"` and asserts the error message references the `:` constraint. This test must fail before the fix.
2. `HandleBundleAdd` checks `strings.Contains(alias, ":")` before calling the use case and returns a descriptive error if true.
3. `TestHandleBundleAdd_RejectsColonInAlias` passes after the fix.
4. All existing tests pass.

---

### FIX-007: Raise test coverage to spec quality gate ≥90% on Use Case layer (P2 — medium)

**Priority:** P2  
**Severity:** Spec quality gate — the spec requires domain+usecase ≥90%; usecase is at 81.5%; MCP adapter at 20.8%

**Problem:** The spec quality gate states "Domain + Use Case layer coverage ≥ 90%." Current coverage:

| Package | Coverage | Gate |
|---|---|---|
| `internal/usecase` | 81.5% | ≥90% |
| `internal/adapter/mcp` | 20.8% | — (secondary) |
| `internal/adapter/okf` | 57.6% | — (secondary) |

Nine of 14 MCP handler functions have 0% coverage. Uncovered use-case paths include error returns in `ReindexBundle`, `WriteConcept`, and the concurrent `appendLog` path.

**Acceptance Criteria:**

1. `go test -coverprofile=coverage.out ./internal/usecase/...` reports ≥90% coverage after all fixes.
2. The following MCP handler functions each have at least one test covering their happy path: `HandleBundleList`, `HandleBundleRemove`, `HandleBundleReindex`, `HandleConceptList`, `HandleConceptLinks`, `HandleIndexRead`, `HandleLogRead`, `HandleConceptTypeList`, `HandleSearchSemantic`, `HandleSearchKeyword`.
3. `domain.ParseScope` (currently 0%) is covered by at least one test for each of the three scope kinds (`global`, `bundle:x`, `path:x:y`) and the error case.
4. Tests are table-driven where appropriate, named after spec sections (`_SpecFR012`, `_SpecFR013`, etc.).
5. All tests pass with `go test -race ./...`.

---

## Non-Goals

The following items were identified during code review but are **explicitly out of scope** for this PRD:

- **`SemanticSearch`/`KeywordSearch` behavioral identity**: Both tools currently use BM25 as the sole embedder, making them functionally identical. This is intentional per ADR-006 (BM25-only for v0.1). The distinction becomes meaningful when ONNX is added in v0.2.
- **`usecase/bundle.go` filesystem coupling**: `AddBundle` calls `os.Stat` + `filepath.WalkDir` directly in the use case layer without a domain filesystem interface. This is a design smell but not a correctness bug. The domain interface extraction is a v0.2 refactor, not a blocker.
- **`pkg/okfcodec/` extraction**: Deferred per ADR-007. No external consumer in v0.1.
- **ONNX embedder**: Deferred per ADR-006. Not a correctness issue for v0.1.

---

## Open Questions

| ID | Question | Owner | Resolution |
|---|---|---|---|
| OQ-001 | FIX-004 concurrency fix approach: three options are valid (bundle-scoped mutex across full WriteConcept flow; append-only WriteReserved variant; move log management into a single locked adapter operation). Which is preferred? | Implementer | **Recommended default: bundle-scoped mutex across full WriteConcept flow.** This preserves Clean Architecture (use case layer owns index/log), is the most straightforward to reason about, and aligns with FIX-002's direction (adapter Put only persists). The append-only WriteReserved variant is also acceptable but requires a new method signature on the repository interface. |

---

## Implementation Guidance

- **TDD mandatory**: Write a failing test for each FIX before writing any production code. Follow Red → Green → Refactor.
- **Code review per fix**: After each FIX is complete, conduct a brief review (architecture, correctness, test quality) before moving to the next.
- **Agent teammates**: Use parallel agent worktrees for independent fixes where possible. FIX-001, FIX-005, and FIX-006 are fully independent and can run in parallel. FIX-003 and FIX-004 depend on FIX-002 completing first.
- **Dependency order**: FIX-002 before FIX-003 (FIX-003 test will fail differently after FIX-002). FIX-004 after FIX-002 (mutex boundary changes with removal of index/log from Put).
- **Do not introduce new Clean Architecture violations**: No adapter or infra imports in domain or usecase.
- **Commit discipline**: Commit after each FIX with a message referencing the FIX ID.

---

