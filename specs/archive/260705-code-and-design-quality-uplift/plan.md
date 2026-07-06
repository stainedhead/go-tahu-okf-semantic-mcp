# Plan: Code & Design Quality Uplift

**Feature:** code-and-design-quality-uplift
**Created:** 2026-07-05
**Status:** Planning

---

## Development Approach

- **SDD+TDD throughout:** spec first, failing test first, then minimum production code to pass, then refactor.
- **Phase-gated:** each phase must have `go test -race ./...` passing clean before the next phase starts.
- **RED tests first:** every bug fix must begin with a test that fails before the fix and passes after.
- **Conventional commits:** `fix:`, `feat:`, `refactor:`, `test:`, `chore:`, `docs:` per AGENTS.md.
- **ADR entries:** every architecturally significant decision → `documents/arch-decisions-record.md`.
- **No force-push:** all commits to `feat/code-and-design-quality-uplift`; auto-merge on CI green.

---

## Phase Breakdown

### Phase 1 — Correctness (High priority, ~4 fixes)

Goal: stop returning wrong results to agents today.

| Task | FR | Test anchor |
|---|---|---|
| B1a: BM25Embedder zero-norm skip | FR-008 | `TestBM25Embedder_ZeroNorm_SkippedOrError` |
| B1b: HNSWStore.Search NaN guard | FR-009 | `TestHNSWStore_ZeroVector_NoNaNScores` |
| B4: ReindexBundle scope-delete | FR-010 | `TestReindexBundle_RemovesStaleChunks` |
| B5: HNSWStore.Load dims+reset | FR-011 | `TestHNSWStore_Load_ValidatesDims` |
| A3: WriteReserved mutex | FR-004 | `TestFileNodeRepository_WriteReserved_ConcurrentNoRace` |

Commit milestone: `fix(retrieval): NaN guard, stale reindex, WriteReserved mutex`

### Phase 2 — Boundary-guard consolidation (~6 changes)

Goal: containment at the boundary that owns it.

| Task | FR | Test anchor |
|---|---|---|
| Create BundlePathResolver | FR-001 | `TestBundlePathResolver_Resolve_RejectsTraversal` |
| Route Get/Put through resolver | FR-002 | existing tests pass |
| Route ReadReserved/WriteReserved | FR-002 | `TestFileNodeRepository_ReadReserved_RejectsTraversal` |
| Route List | FR-003 | `TestFileNodeRepository_List_RejectsTraversal` |
| Symlink guard on target | FR-006 | `TestBundlePathResolver_SymlinkEscape_Rejected` |
| Consolidate ValidatePath | FR-005 | existing handler tests pass |
| Use-case path validation in WriteConcept | FR-007 | `TestConceptService_WriteConcept_RejectsTraversal` |

Commit milestone: `feat(security): bundle path resolver consolidates all containment checks`

### Phase 3 — Operational hardening (~8 changes)

| Task | FR | Test anchor |
|---|---|---|
| HTTP timeouts | FR-020 | `TestServeHTTP_Healthz` (httptest) |
| Non-loopback block | FR-021 | `TestServeHTTP_NonLoopback_Rejected` |
| stdio ctx shutdown | FR-022 | manual verification + log output |
| Panic recovery in middleware | FR-023 | `TestLoggingMiddleware_PanicRecovered` |
| Read caps (LimitReader) | FR-024 | `TestFileNodeRepository_Get_LargeFile_Capped` |
| gosec to golangci.yml | FR-025 | CI green with gosec |
| SHA-pin GitHub Actions | FR-026 | manual review |
| Integration CI job | FR-027 | `//go:build integration` test + CI job |
| Config validation | FR-028 | `TestConfig_InvalidPort_ReturnsError` |
| flock registry | FR-028 | `TestYAMLBundleRepository_CrossProcess_NoLostUpdates` |

Commit milestone: `feat(hardening): HTTP timeouts, non-loopback block, flock registry, gosec`

### Phase 4 — Honesty & retrieval quality (~7 changes)

| Task | FR | Test anchor |
|---|---|---|
| EmbeddingModel startup error | FR-015 | `TestBuildServices_UnknownEmbedModel_Errors` |
| --config flag respected | FR-016 | `TestServe_ConfigFlag_ReadsSpecifiedFile` |
| Server version from ldflags | FR-019 | `TestMCPServer_VersionMatchesBuild` |
| Vocab IDF selection | FR-012 | `TestBM25Embedder_VocabByIDF_NotDF` |
| Distinguish Semantic/Keyword | FR-013 | `TestSearchService_Semantic_KeywordAreDistinct` |
| Scope boundary fix | FR-014 | `TestHNSWStore_ScopePath_NoLeakage` |
| Doc reconciliation (AGENTS.md) | FR-017 | grep-based CI check or manual |

Commit milestone: `fix(honesty): EmbeddingModel errors, --config respected, IDF vocab, scope fix`

### Phase 5 — Test uplift & domain hardening (~10 changes)

| Task | FR | Test anchor |
|---|---|---|
| NewConceptRef constructor | FR-029 | `TestNewConceptRef_Validates` |
| OKFFrontmatter.Validate | FR-030 | `TestOKFFrontmatter_Validate_RequiresType` |
| Clock injection | FR-031 | `TestAppendLog_DeterministicTimestamp` |
| domaintest package | FR-032 | imported by usecase+mcp tests |
| Delete indexer.go | FR-018 | all tests still pass |
| infra/config tests to ≥80% | coverage | `TestConfig_*` suite |
| infra/registry tests to ≥80% | coverage | `TestYAMLBundleRepository_*` suite |
| infra/transport tests to ≥60% | coverage | `TestServeHTTP_*`, middleware tests |
| adapter/okf tests to ≥70% | coverage | List/ReadReserved/WriteReserved/ListTypes |
| Integration test | FR-027 | `//go:build integration` round-trip test |
| CI gate ratchet | NFR | `.github/workflows/ci.yml` coverage step |

Commit milestone: `test(uplift): coverage floors met, domaintest, validating constructors, indexer deleted`

---

## Critical Path

Phase 1 → Phase 2 → Phase 3/4 (parallel if bandwidth allows) → Phase 5

Phase 1 is strictly prerequisite because the NaN bug affects test reliability. Phase 2 is prerequisite to Phase 3/4's path-related tests. Phase 5 can only delete `indexer.go` after Phase 3/4 tests replace its coverage.

---

## Testing Strategy

- Every bug fix: RED test first, then fix, then GREEN.
- Every new component: test in the same package (black-box `_test` package where possible).
- Concurrency: `go test -race` after every Phase gate.
- Integration tests: `//go:build integration` + `t.TempDir()` for filesystem isolation.
- Coverage: measure after each phase; ratchet gates only in Phase 5.

---

## Rollout Strategy

All work on `feat/code-and-design-quality-uplift`. Single PR to `main` at the end. Auto-merge once CI green. No feature flags needed — all fixes improve correctness/safety without new user-facing behavior (except the `EmbeddingModel` error, `--config` respect, and non-loopback block, which are correctly flagged as breaking changes in the spec).

---

## Success Metrics

- `go test -race ./...` passes clean throughout
- Coverage floors met (see NFRs)
- `gosec` clean in CI
- All 25 named acceptance criteria in spec.md checked off
- PR opened; merged by user after review
