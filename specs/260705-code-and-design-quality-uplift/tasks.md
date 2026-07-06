# Tasks: Code & Design Quality Uplift

**Feature:** code-and-design-quality-uplift
**Created:** 2026-07-05
**Status:** Planning

---

## Progress Summary

**12 / 35 tasks complete**

---

## Phase 1 — Correctness (5 tasks)

| ID | Task | Deps | Est (h) | Status | Acceptance Criteria |
|---|---|---|---|---|---|
| P1.1 | Write RED test `TestHNSWStore_ZeroVector_NoNaNScores` | — | 0.5 | ✅ | Test fails; confirms NaN score in search result |
| P1.2 | Fix: BM25Embedder skip zero-norm vectors (FR-008) | P1.1 | 1 | ✅ | Store-level guard (isZeroNorm in Upsert) is embedder-agnostic fix |
| P1.3 | Fix: HNSWStore.Search NaN guard (FR-009) | P1.1 | 1 | ✅ | `TestHNSWStore_ZeroVector_NoNaNScores` passes; no NaN in results |
| P1.4 | Write RED test + fix: ReindexBundle scope-delete (FR-010) | — | 1.5 | ✅ | `TestReindexBundle_RemovesStaleChunks` passes; deleted concept absent from search |
| P1.5 | Write RED test + fix: HNSWStore.Load dims+reset (FR-011) | — | 1 | ✅ | `TestHNSWStore_Load_ValidatesDims` passes; Load resets state; dims validated |
| P1.6 | Fix: WriteReserved acquires f.mu (FR-004) + race test | — | 0.5 | ✅ | `go test -race` passes; `TestFileNodeRepository_WriteReserved_ConcurrentNoRace` passes |

**Phase 1 gate:** `go test -race ./...` green; all RED tests present and passing.

---

## Phase 2 — Boundary-guard consolidation (7 tasks)

| ID | Task | Deps | Est (h) | Status | Acceptance Criteria |
|---|---|---|---|---|---|
| P2.1 | Create `BundlePathResolver` + tests (FR-001) | Phase 1 | 2 | ✅ | `TestBundlePathResolver_Resolve_RejectsTraversal` passes; symlink test passes |
| P2.2 | Route `Get`/`Put` through resolver (FR-002) | P2.1 | 1 | ✅ | Existing repository tests pass; no regression |
| P2.3 | Route `ReadReserved`/`WriteReserved` through resolver (FR-002) | P2.1 | 1 | ✅ | `TestFileNodeRepository_ReadReserved_RejectsTraversal` passes |
| P2.4 | Route `List` through resolver (FR-003) | P2.1 | 0.5 | ✅ | `TestFileNodeRepository_List_RejectsTraversal` passes |
| P2.5 | EvalSymlinks on target in resolver (FR-006) | P2.1 | 1 | ✅ | `TestBundlePathResolver_SymlinkEscape_Rejected` passes; covered by P2.1 resolver |
| P2.6 | Consolidate `ValidatePath` → resolver (FR-005) | P2.1 | 0.5 | ✅ | Handler tests pass; `validation.go` comment updated to defense-in-depth role |
| P2.7 | Use-case path validation in `WriteConcept` (FR-007) | P2.1 | 0.5 | ✅ | `TestConceptService_WriteConcept_RejectsTraversal` passes |

**Phase 2 gate:** `go test -race ./...` green; traversal tests present and passing.

---

## Phase 3 — Operational hardening (10 tasks)

| ID | Task | Deps | Est (h) | Status | Acceptance Criteria |
|---|---|---|---|---|---|
| P3.1 | HTTP server timeouts (FR-020) + `TestServeHTTP_Healthz` | Phase 2 | 1 | ⬜ | ReadHeaderTimeout ≥ 5s set; healthz 200/405 test passes |
| P3.2 | Non-loopback block at startup (FR-021) | Phase 2 | 1 | ⬜ | `--bind 0.0.0.0` returns error; `--bind 127.0.0.1` still works |
| P3.3 | stdio ctx shutdown (FR-022) | Phase 2 | 1 | ⬜ | SIGTERM unblocks stdio serve; index persisted on signal |
| P3.4 | Panic recovery in middleware (FR-023) + test | Phase 2 | 1 | ⬜ | `TestLoggingMiddleware_PanicRecovered` passes; daemon not crashed |
| P3.5 | Read caps `io.LimitReader` on `os.ReadFile` paths (FR-024) | Phase 2 | 1 | ⬜ | Large-file test returns error; no memory exhaustion |
| P3.6 | Add `gosec` to `.golangci.yml` (FR-025) | Phase 2 | 0.5 | ⬜ | `golangci-lint run ./...` passes with gosec enabled |
| P3.7 | SHA-pin GitHub Actions (FR-026) | Phase 2 | 0.5 | ⬜ | All `@vN` tags replaced with commit SHAs + version comment |
| P3.8 | Integration CI job (FR-027) | Phase 2 | 1 | ⬜ | `.github/workflows/ci.yml` has integration job; one `//go:build integration` test exists |
| P3.9 | Config field validation (FR-028) | Phase 2 | 1 | ⬜ | `TestConfig_InvalidPort_ReturnsError` passes; bad values rejected |
| P3.10 | flock registry (FR-028) | Phase 2 | 1.5 | ⬜ | `TestYAMLBundleRepository_RoundTrip` passes; flock held during save |

**Phase 3 gate:** `go test -race ./...` green; `golangci-lint` with gosec clean.

---

## Phase 4 — Honesty & retrieval quality (7 tasks)

| ID | Task | Deps | Est (h) | Status | Acceptance Criteria |
|---|---|---|---|---|---|
| P4.1 | EmbeddingModel startup error (FR-015) | Phase 3 | 1 | ⬜ | Unknown model → non-zero exit with clear message |
| P4.2 | `--config` flag respected (FR-016) | Phase 3 | 0.5 | ⬜ | `--config /tmp/x.yaml` reads that file |
| P4.3 | Server version from ldflags (FR-019) | Phase 3 | 0.5 | ⬜ | MCP server registration uses `main.version` |
| P4.4 | Vocab IDF selection (FR-012) | Phase 3 | 2 | ⬜ | `TestBM25Embedder_VocabByIDF` passes; high-DF common terms deprioritized |
| P4.5 | Distinguish SemanticSearch/KeywordSearch (FR-013) | Phase 3 | 1 | ⬜ | `TestSearchService_Semantic_KeywordAreDistinct` passes; two embedder fields |
| P4.6 | ScopePath boundary + empty subpath (FR-014) | Phase 3 | 1 | ⬜ | `TestHNSWStore_ScopePath_NoLeakage` passes; `ParseScope("path:b:")` errors |
| P4.7 | Doc reconciliation: AGENTS.md + documents/ (FR-017) | Phase 3 | 1 | ⬜ | `grep -r "GraphRepository\|Node/Edge/Facet" AGENTS.md` → empty |

**Phase 4 gate:** `go test -race ./...` green; all config/retrieval tests passing.

---

## Phase 5 — Test uplift & domain hardening (10 tasks)

| ID | Task | Deps | Est (h) | Status | Acceptance Criteria |
|---|---|---|---|---|---|
| P5.1 | `NewConceptRef` validating constructor (FR-029) | Phase 4 | 1 | ⬜ | `TestNewConceptRef_Validates` passes; invalid refs rejected |
| P5.2 | `OKFFrontmatter.Validate()` method (FR-030) | Phase 4 | 0.5 | ⬜ | `TestOKFFrontmatter_Validate_RequiresType` passes |
| P5.3 | Clock injection in BundleService/ConceptService (FR-031) | Phase 4 | 1 | ⬜ | `TestAppendLog_DeterministicTimestamp` passes |
| P5.4 | `internal/domain/domaintest/` package (FR-032) | Phase 4 | 1.5 | ⬜ | Package importable; usecase+mcp tests use it; duplicates removed |
| P5.5 | Delete `indexer.go` (FR-018) | P5.4 | 0.5 | ⬜ | File gone; all tests still pass |
| P5.6 | `infra/config` tests to ≥ 80% | Phase 4 | 1.5 | ⬜ | `go test -cover` reports ≥ 80% |
| P5.7 | `infra/registry` tests to ≥ 80% | Phase 4 | 1.5 | ⬜ | `go test -cover` reports ≥ 80% |
| P5.8 | `infra/transport` tests to ≥ 60% | Phase 4 | 1 | ⬜ | `go test -cover` reports ≥ 60% |
| P5.9 | `adapter/okf` tests to ≥ 70% | Phase 4 | 1.5 | ⬜ | `go test -cover` reports ≥ 70% |
| P5.10 | CI coverage gate ratchet | P5.6–5.9 | 0.5 | ⬜ | `.github/workflows/ci.yml` enforces new floor thresholds |

**Phase 5 gate:** All coverage floors met; `indexer.go` deleted; `go test -race ./...` green; `golangci-lint` clean.

---

## Task Status Legend

| Symbol | Meaning |
|---|---|
| ⬜ | Not started |
| 🔄 | In progress |
| ✅ | Complete |
| ❌ | Blocked / failed |
