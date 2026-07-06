# PRD: Code & Design Quality Uplift

**Created:** 2026-07-05
**Jira:** N/A
**Status:** Draft
**Source:** RFC-001-code-and-design-quality.md

---

## Problem Statement

`tahu` is a solid v0.1 foundation with correct Clean Architecture layering, consistent `%w` error wrapping, and genuinely table-driven tests. However, ~30 findings across four themes make it fragile to evolve safely:

- **Correctness defects return wrong search results to agents today.** Zero-vector BM25 embeddings produce `NaN` similarity scores that corrupt ranking (reproduced). `ReindexBundle` never deletes stale vectors, so deleted/renamed concepts surface in results forever.
- **Security/containment invariants are guarded at one caller, not at the boundary that owns them.** `ReadReserved`/`WriteReserved` skip the containment check that `Get`/`Put` enforce — safe only because today's one caller happens to pre-validate. Any new caller reintroduces arbitrary file read/write.
- **Advertised capabilities are silent no-ops.** `TAHU_EMBED_MODEL` / `embedding_model` config is read but never consumed; BM25 is hard-wired. `serve --config <path>` is accepted then discarded. Both were reproduced empirically.
- **The outer infra/cmd layers are near-zero in test coverage** (`infra/config` and `infra/registry` at 0%) despite conventions requiring ≥90%, and no integration tests exist despite `AGENTS.md` promising them.

The unifying narrative: **invariants are guarded at one layer instead of the boundary that owns them.** Fixing this structurally — not case-by-case — is the highest-leverage change.

---

## Goals

- Eliminate the correctness defects that return wrong search results to agents today
- Move security/containment invariants to the boundary that owns them so future callers can't bypass them
- Close the "advertised but not implemented" gap between docs/config and running behavior
- Raise test coverage in the outer layers and add the integration tests the conventions already promise

---

## Non-Goals

- Replacing BM25 with a true dense embedder (ONNX/MiniLM) — a separate product decision; this PRD only makes the seam honest and correct
- Windows support (deferred, NG8)
- Full multi-tenant / networked hardening beyond making HTTP mode safe-by-default and clearly documented

---

## Functional Requirements

### Theme A — Boundary-guard consolidation

**FR-001:** Introduce a single "bundle path resolver" that every filesystem access goes through, returning a validated absolute path or an error. It must be the only way to convert `(bundleAlias, relPath)` into a real filesystem path.

**FR-002:** `FileNodeRepository.ReadReserved` and `WriteReserved` must enforce `ValidateConceptPath`-style containment internally — they must not rely on callers to pre-validate.

**FR-003:** `FileNodeRepository.List` must enforce bundle-root containment on the `subPath` argument before walking the filesystem.

**FR-004:** `WriteReserved` must acquire `f.mu` before writing. The repository's stated invariant ("mu serializes writes across all bundles") must hold for all write methods, not just `Put`.

**FR-005:** `ValidatePath` (adapter/mcp) and `ValidateConceptPath` (adapter/okf) must be consolidated into one canonical path-validation routine. The weaker parallel implementation must be retired.

**FR-006:** `ValidateConceptPath` must canonicalize the resolved target (via `filepath.EvalSymlinks` or `Lstat`-and-reject) — not only the bundle root — before the prefix containment check, so symlinks inside the bundle cannot escape it.

**FR-007:** `ConceptService.WriteConcept` must validate `ref.RelativePath` (non-empty, `.md` extension, no `..`) at the use-case layer as defense-in-depth, independent of adapter-boundary checks.

### Theme B — Retrieval correctness

**FR-008:** `BM25Embedder.Embed` must not return zero-norm vectors to the vector store. Zero-norm input must be detected and either skipped (no chunk indexed) or returned as an explicit error.

**FR-009:** `HNSWStore.Search` must guard against `NaN` scores before sorting. A zero-vector query must return an empty result set, not a `NaN`-corrupted ranking.

**FR-010:** `ReindexBundle` must delete all existing vectors scoped to the bundle (or diff against listed refs) before upserting the current concepts. A "full reindex" must not accumulate stale vectors.

**FR-011:** `HNSWStore.Load` must validate that the persisted graph's dimensionality matches the configured `dims` and must reset `graph` and `chunks` to empty before loading, not merge into existing state.

**FR-012:** `buildVocab` vocabulary selection must favor IDF discriminativeness over document frequency. The highest-DF terms must not crowd out rare, high-IDF terms that retrieval depends on.

**FR-013:** `SemanticSearch` and `KeywordSearch` must be distinct capabilities. Either give `SearchService` two separate embedder fields (`DenseEmbedder`, `KeywordEmbedder`) or collapse to one method with a mode parameter and remove the misleading comment.

**FR-014:** `ScopePath` filtering must add a path-separator boundary to the `strings.HasPrefix` check so `path:kb:foo` does not match `foobar/`. `ParseScope` must reject an empty sub-path with a wrapped sentinel error.

### Theme C — Advertised vs. real

**FR-015:** `config.EmbeddingModel` must either be honored in `buildServices` (select embedder by value, return error on unknown) or removed from `Config` until ONNX support lands. Silent no-op is not acceptable. `EmbeddingBatchSize` is subject to the same rule.

**FR-016:** `serve --config <path>` must pass the specified path into `config.Load`. If the flag is not wired, it must be removed from the command definition.

**FR-017:** `AGENTS.md` and all files under `documents/` must be reconciled with the real Concept-based domain model (`OKFConcept`, `BundleEntry`, `EmbeddingChunk`, `ConceptRef`). References to `Node`/`Edge`/`Facet`/`Graph`/`GraphRepository` must be removed or replaced.

**FR-018:** `internal/adapter/okf/indexer.go` (`GenerateIndex`, `AppendLog`) must be deleted or the use case must delegate to it. Two divergent implementations of the same feature must not coexist.

**FR-019:** The MCP server version registered in `transport.NewMCPServer` must reflect the ldflags-injected `main.version`, not the hardcoded `"0.1.0"`.

### Theme D — Operational hardening

**FR-020:** The HTTP server must set `ReadHeaderTimeout`, `ReadTimeout`, `IdleTimeout`, and `MaxHeaderBytes`. The SSE long-lived write endpoint may have a relaxed or absent `WriteTimeout` scoped to that route only.

**FR-021:** HTTP mode must require a bearer token or shared secret when `--bind` is non-loopback. Non-loopback binds without explicit auth configuration must be rejected at startup with a clear error. Stdio users must be unaffected.

**FR-022:** `ServeStdio` must select on `ctx.Done()` and initiate shutdown when the context is cancelled, so SIGINT/SIGTERM unblocks the process and the vector index is persisted via the deferred `store.Persist`. Alternatively, the `ctx` parameter must be removed and the EOF-only shutdown contract documented.

**FR-023:** The tool-handler middleware must include a deferred `recover()` that logs the panic details and returns an error `CallToolResult` rather than crashing the daemon.

**FR-024:** All `os.ReadFile` calls on the read path (`repository.Get`, `repository.ListTypes`, and any remaining indexer paths) must be capped. Implement via `stat`-and-reject above a configurable limit or `io.LimitReader`. HTTP mode must additionally wrap handlers with `http.MaxBytesReader` or `http.MaxBytesHandler`.

**FR-025:** `gosec` must be added to `.golangci.yml` as an enabled linter. All `gosec` violations introduced by this work must be resolved before merge.

**FR-026:** GitHub Actions workflow actions must be pinned to full commit SHAs (with a version comment). Tag-only pins (`@v4`, `@v5`, etc.) are not acceptable given CD's `contents: write` permission.

**FR-027:** Integration tests under `//go:build integration` must exist in `internal/adapter/` and must be run in CI via a dedicated job (e.g., `go test -tags integration ./...`).

**FR-028:** `config.Load` must validate all fields and return or log an error on invalid values (e.g., unknown transport, out-of-range port). Silent fallback to defaults is not acceptable for values the operator explicitly set.

### Domain modeling (cross-cutting)

**FR-029:** Domain value objects (`ConceptRef`, `BundleEntry`, `EmbeddingChunk`, `Scope`) must have validating constructors (`NewConceptRef`, etc.) that enforce non-empty, `.md` extension, and no-`:` constraints. Direct struct literal construction of invalid states must be eliminated from production code.

**FR-030:** The `type`-required invariant must become a method on `OKFFrontmatter` (e.g., `Validate() error`) rather than an ad-hoc check in `WriteConcept`.

**FR-031:** `AddBundle`, `ReindexBundle`, and `appendLog` must accept a `clock func() time.Time` injection (or equivalent) so timestamps are deterministic in tests. `time.Now()` must not be called directly in the use-case layer.

**FR-032:** `internal/domain/fake_test.go` must be promoted to `internal/domain/domaintest/` as an importable package. Near-identical fake declarations in `usecase` and `mcp` test packages must be removed and replaced with the shared fakes.

---

## Non-Functional Requirements

- **Performance:** No regression in search latency for existing bundles. The vocab-selection change (FR-012) must land with a fixed evaluation set so retrieval quality change is measured, not guessed.
- **Reliability:** Zero-vector queries must return empty results, not `NaN`-corrupted rankings. Stale vectors must be removed on every reindex. `go test -race ./...` must pass clean after every phase.
- **Security:** Path containment enforced at the repository boundary (not only at the handler). HTTP transport authentication required for non-loopback binds. No symlink escapes from bundle roots. GitHub Actions supply chain secured via SHA pinning.
- **Observability:** Config validation errors surfaced at startup with actionable messages (not silent fallback). `gosec` enforced in CI. Structured JSON logging continues to stderr; no absolute filesystem paths logged as error details.
- **Test coverage floors (post-uplift targets):**
  - `internal/infra/config`: ≥ 80%
  - `internal/infra/registry`: ≥ 80%
  - `internal/infra/transport`: ≥ 60%
  - `internal/adapter/okf`: ≥ 70%
  - `internal/adapter/vectorstore`: ≥ 85%
  - `internal/adapter/mcp`: ≥ 75%
  - `internal/domain` and `internal/usecase`: maintain ≥ 90%

---

## Acceptance Criteria

### Correctness
- [ ] `TestHNSWStore_ZeroVector_NoNaNScores` fails before the fix (RED) and passes after (GREEN)
- [ ] `TestReindexBundle_RemovesStaleChunks` fails before the fix and passes after; a deleted concept no longer appears in search results after reindex
- [ ] `TestHNSWStore_Load_ValidatesDims` fails when persisted dims differ from configured dims
- [ ] `go test -race ./...` passes clean across all packages after every phase

### Security / boundary guards
- [ ] `TestFileNodeRepository_ReadReserved_RejectsTraversal` fails before FR-002 is implemented and passes after; `ReadReserved("../../etc/passwd")` returns `domain.ErrPathEscape`
- [ ] `TestFileNodeRepository_WriteReserved_RejectsTraversal` same as above for writes
- [ ] A symlink inside a bundle root pointing to a file outside the root is rejected with `ErrPathEscape` on read

### Config / advertised capabilities
- [ ] `TAHU_EMBED_MODEL=minilm-l6-v2 tahu serve` returns a non-zero exit with a clear "unimplemented embedding model" error (or the model is actually supported)
- [ ] `tahu serve --config /tmp/custom.yaml` reads `/tmp/custom.yaml`, not `~/.tahu/config.yaml`
- [ ] `TestConfig_EnvOverridePrecedence` confirms env vars beat config file values for all overridable fields
- [ ] `TestConfig_InvalidPort_ReturnsError` confirms malformed `TAHU_PORT` is rejected, not silently ignored

### Operational hardening
- [ ] `TestServeHTTP_Healthz` verifies `GET /healthz` → 200 and `POST /healthz` → 405
- [ ] `TestLoggingMiddleware_PopulatesRequestID` verifies the `request_id` field is present and non-empty in tool-call log output
- [ ] HTTP server construction sets `ReadHeaderTimeout` ≥ 5s; verified by inspection or test
- [ ] `tahu serve --transport http --bind 0.0.0.0` without auth config fails at startup with a clear error
- [ ] A panic in a tool handler is recovered, logged, and returned as an error `CallToolResult` (not a daemon crash)

### Documentation / dead code
- [ ] `AGENTS.md` contains no references to `Node`, `Edge`, `Facet`, `Graph`, or `GraphRepository`
- [ ] `internal/adapter/okf/indexer.go` does not exist (deleted), or the use case delegates to it exclusively
- [ ] `internal/domain/domaintest/` package exists and is imported by `usecase` and `mcp` test packages; no duplicate fake declarations remain

### Test coverage
- [ ] `TestYAMLBundleRepository_RoundTrip` — `Put` → save → load → `Get` returns equal entry
- [ ] `TestYAMLBundleRepository_ConcurrentPut_NoRace` — concurrent mutations pass `-race`
- [ ] `TestHNSWStore_Delete_RemovesChunk` — upsert, delete, search confirms removal
- [ ] `TestChunkConcept_MultibyteRunes` — chunking on CJK/emoji splits by rune, not byte
- [ ] At least one test file exists under `internal/adapter/` with `//go:build integration` and runs in CI
- [ ] Per-package coverage floors in NFRs are met (verified by `go test -cover ./...` in CI)

---

## Dependencies and Risks

| Item | Type | Notes |
|------|------|-------|
| `github.com/coder/hnsw` — `CosineDistance` returns `NaN` for zero vector | Dependency | Confirmed empirically; fix is in our layer (skip/guard zero vectors), not the library |
| HTTP auth mechanism choice (bearer token vs. shared secret vs. mTLS) | Risk | Must not break existing local stdio orchestration; gate behind config with loopback-only default |
| Vocab-selection change (FR-012) alters search result rankings | Risk | Land with a fixed evaluation set; measure quality delta before merge |
| Deleting `indexer.go` (FR-018) removes tests that currently contribute to adapter/okf coverage | Risk | Expected regression is intentional; net auditability improves; new direct tests replace the lost coverage |
| Consolidated path resolver (FR-001) touches every filesystem call | Risk | Must be landed behind the existing repository tests plus the new traversal tests to prevent regressions |
| Cross-process registry write races (two `tahu` processes) | Dependency / Risk | In-process mutex prevents intra-process races; advisory file lock or documented single-writer contract needed for multi-process safety |
| GitHub Actions SHA pinning requires tracking upstream releases manually | Dependency | Use tooling (e.g., `pin-github-actions` or Dependabot) to automate future updates |

---

## Open Questions

_All resolved 2026-07-05._

- **Q1 — EmbeddingModel (FR-015):** ✅ Return a startup error when an unsupported model is configured. Do not silently fall back to BM25.
- **Q2 — HTTP auth (FR-021):** ✅ Block non-loopback binds at startup with a clear error. No bearer token or shared-secret scheme for now; defer full auth to a future story.
- **Q3 — `indexer.go` deletion (FR-018):** ✅ Delete in Phase 5 (test uplift), after new tests replace the coverage it currently contributes.
- **Q4 — Registry lock (FR-028):** ✅ Use `flock`-based advisory file lock around load→save for cross-process safety.
