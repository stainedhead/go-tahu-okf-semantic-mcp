# Spec: Code & Design Quality Uplift

**Feature:** code-and-design-quality-uplift
**Created:** 2026-07-05
**Status:** Draft
**Source PRD:** specs/260705-code-and-design-quality-uplift/code-and-design-quality-uplift-PRD.md

---

## Executive Summary

`tahu` is a well-structured v0.1 codebase with correct Clean Architecture layering and consistent error handling. This uplift resolves ~30 findings across four themes that make the codebase fragile to evolve: correctness defects that return wrong search results today, security/containment invariants guarded at the wrong layer, advertised capabilities that are silent no-ops, and outer-layer test coverage near zero. The work is sequenced across five implementation phases, each independently shippable, always TDD-first.

---

## Problem Statement

Four themes of systemic fragility were identified in a layer-by-layer review:

- **Retrieval correctness:** Zero-vector BM25 embeddings produce `NaN` similarity scores (reproduced). `ReindexBundle` never deletes stale vectors, so dead concepts surface in search results indefinitely.
- **Boundary-guard asymmetry:** `ReadReserved`/`WriteReserved` skip the containment check that `Get`/`Put` enforce. Safe only because today's one caller pre-validates — any future caller reintroduces arbitrary file read/write.
- **Advertised vs. real:** `TAHU_EMBED_MODEL` / `embedding_model` config is read but never consumed (reproduced). `serve --config <path>` is accepted then silently discarded.
- **Outer-layer coverage:** `infra/config` and `infra/registry` at 0% coverage; no integration tests exist despite `AGENTS.md` promising them.

The unifying narrative: invariants are guarded at one layer instead of the boundary that owns them.

---

## Goals

1. Eliminate correctness defects that return wrong search results to agents today
2. Move security/containment invariants to the boundary that owns them so future callers can't bypass them
3. Close the "advertised but not implemented" gap between docs/config and running behavior
4. Raise test coverage in the outer layers and add the integration tests the conventions already promise

---

## Non-Goals

- Replacing BM25 with a true dense embedder (ONNX/MiniLM) — separate product decision
- Windows support (deferred, NG8)
- Full multi-tenant / networked hardening beyond making HTTP mode safe-by-default

---

## Functional Requirements

### Theme A — Boundary-guard consolidation

**FR-001:** Introduce a single "bundle path resolver" — the only way to convert `(bundleAlias, relPath)` into a validated absolute path or an error.

**FR-002:** `FileNodeRepository.ReadReserved` and `WriteReserved` must enforce `ValidateConceptPath`-style containment internally.

**FR-003:** `FileNodeRepository.List` must enforce bundle-root containment on `subPath` before walking.

**FR-004:** `WriteReserved` must acquire `f.mu` before writing.

**FR-005:** `ValidatePath` and `ValidateConceptPath` must be consolidated into one canonical routine.

**FR-006:** `BundlePathResolver` must guard against symlink escapes. For **existing** paths use `filepath.EvalSymlinks` on the final resolved path and re-apply the prefix check. For **non-existent** paths (new concept writes), use `os.Lstat` on the final path component: if it returns no error and the entry is a symlink, reject with `ErrPathEscape`; if it returns `os.ErrNotExist`, the path is new and safe to create. Never call `EvalSymlinks` on a path that may not yet exist.

**FR-007:** `ConceptService.WriteConcept` must validate `ref.RelativePath` at the use-case layer as defense-in-depth.

### Theme B — Retrieval correctness

**FR-008:** `BM25Embedder.Embed` must not return zero-norm vectors; detect and skip or error.

**FR-009:** `HNSWStore.Search` must guard against `NaN` scores; zero-vector query returns empty, not corrupted ranking.

**FR-010:** `ReindexBundle` must scope-delete existing bundle chunks before upserting current concepts.

**FR-011:** `HNSWStore.Load` must validate persisted dims match configured dims; reset graph+chunks before loading.

**FR-012:** `buildVocab` must select vocabulary by IDF discriminativeness, not document frequency.

**FR-013:** `SemanticSearch` and `KeywordSearch` must be distinct capabilities (separate embedder fields or mode param).

**FR-014:** `ScopePath` filtering must add a path-separator boundary; `ParseScope` must reject empty sub-path.

### Theme C — Advertised vs. real

**FR-015:** `config.EmbeddingModel` unsupported value → startup error (not silent BM25 fallback). *(Resolved: return error.)*

**FR-016:** `serve --config <path>` must pass the path into `config.Load` or be removed.

**FR-017:** `AGENTS.md` and `documents/` must be reconciled with the real Concept-based domain model; remove all Node/Edge/Facet/Graph/GraphRepository references.

**FR-018:** `internal/adapter/okf/indexer.go` must be deleted in Phase 5 after new tests replace its coverage contribution. *(Resolved: Phase 5.)*

**FR-019:** MCP server version must reflect the ldflags-injected `main.version`.

### Theme D — Operational hardening

**FR-020:** HTTP server must set `ReadHeaderTimeout`, `ReadTimeout`, `IdleTimeout`, `MaxHeaderBytes`.

**FR-021:** Non-loopback HTTP binds must be rejected at startup with a clear error. *(Resolved: block non-loopback; no bearer/shared-secret scheme now.)*

**FR-022:** `ServeStdio` must select on `ctx.Done()` for graceful shutdown and index persistence.

**FR-023:** Tool-handler middleware must include a deferred `recover()`.

**FR-024:** All `os.ReadFile` calls on the read path must be capped at **1 MB** (matching `MaxBodyBytes` on the write path). Implement via `stat`-and-reject: if `os.Stat` reports `Size > 1 MB`, return a wrapped `domain.ErrInputTooLarge` before reading. HTTP transport additionally wraps with `http.MaxBytesReader` at 2 MB (1 MB content + overhead).

**FR-025:** `gosec` must be added to `.golangci.yml`; all violations resolved.

**FR-026:** GitHub Actions must be pinned to full commit SHAs.

**FR-027:** Integration tests under `//go:build integration` must exist and run in CI.

**FR-028:** `config.Load` must validate all fields and return error on invalid values. Registry uses `flock`-based advisory lock around load→save. *(Resolved: flock.)*

### Domain modeling

**FR-029:** Domain value objects must have validating constructors (`NewConceptRef`, etc.).

**FR-030:** `type`-required invariant becomes a method on `OKFFrontmatter`.

**FR-031:** `AddBundle`, `ReindexBundle`, `appendLog` accept `clock func() time.Time` for deterministic tests.

**FR-032:** `internal/domain/fake_test.go` promoted to `internal/domain/domaintest/`; duplicates removed.

---

## Non-Functional Requirements

- **Performance:** No regression in search latency. Vocab-selection change ships with a fixed evaluation set.
- **Reliability:** Zero-vector queries return empty, not NaN. Stale vectors removed on every reindex. `go test -race ./...` passes clean after every phase.
- **Security:** Path containment at repository boundary. Non-loopback HTTP blocked at startup. No symlink escapes from bundle roots. GitHub Actions SHA-pinned.
- **Observability:** Config validation errors surfaced at startup. `gosec` enforced in CI. No filesystem paths logged as error details.
- **Coverage floors (post-uplift):**
  - `internal/infra/config`: ≥ 80%
  - `internal/infra/registry`: ≥ 80%
  - `internal/infra/transport`: ≥ 60%
  - `internal/adapter/okf`: ≥ 70%
  - `internal/adapter/vectorstore`: ≥ 85%
  - `internal/adapter/mcp`: ≥ 75%
  - `internal/domain`, `internal/usecase`: maintain ≥ 90%

---

## System Architecture

### Affected layers

| Layer | Impact |
|---|---|
| `internal/domain` | FR-029–032: validating constructors, `OKFFrontmatter.Validate()`, clock injection, domaintest package |
| `internal/usecase` | FR-007, FR-010, FR-013, FR-031: WriteConcept validation, ReindexBundle delete, search distinction, clock injection |
| `internal/adapter/okf` | FR-001–006: path resolver, containment in ReadReserved/WriteReserved/List, mutex fix, symlink guard |
| `internal/adapter/embedder` | FR-008, FR-012: zero-norm guard, IDF-based vocab selection |
| `internal/adapter/vectorstore` | FR-009, FR-011: NaN guard, Load reset+dims check |
| `internal/adapter/mcp` | FR-005: consolidate ValidatePath |
| `internal/infra/config` | FR-015, FR-016, FR-028: EmbeddingModel validation, --config flag, field validation |
| `internal/infra/registry` | FR-028: flock advisory lock |
| `internal/infra/transport` | FR-020–023: HTTP timeouts, non-loopback block, stdio ctx shutdown, panic recovery |
| `cmd/tahu` | FR-015, FR-016, FR-019: wiring EmbeddingModel, config path, server version |
| `.github/` | FR-026, FR-027: SHA-pin actions, integration CI job |
| `.golangci.yml` | FR-025: add gosec |
| `AGENTS.md`, `documents/` | FR-017: doc reconciliation |
| `internal/adapter/okf/indexer.go` | FR-018: delete in Phase 5 |

### New components

- `internal/adapter/okf/pathresolver.go` — canonical bundle path resolver (FR-001)
- `internal/domain/domaintest/` — shared fake implementations (FR-032)
- `internal/infra/registry/flock.go` — advisory file lock wrapper (FR-028)

---

## Scope of Changes

### Files to create
- `internal/adapter/okf/pathresolver.go` + `pathresolver_test.go`
- `internal/domain/domaintest/fakes.go`
- `internal/infra/registry/flock.go` + `flock_test.go`
- `internal/infra/config/config_test.go`
- `internal/infra/registry/yaml_test.go`
- `internal/infra/transport/server_test.go` (expand)
- `internal/adapter/vectorstore/hnsw_nan_test.go`
- `internal/adapter/okf/repository_extended_test.go`
- Integration test files: `internal/adapter/okf/repository_integration_test.go`

### Files to modify (non-exhaustive)
- `internal/adapter/okf/repository.go` — A1–A4 fixes
- `internal/adapter/okf/validator.go` — A5 symlink fix
- `internal/adapter/mcp/validation.go` — A4 consolidation
- `internal/adapter/embedder/bm25.go` — B1, B2 fixes
- `internal/adapter/vectorstore/hnsw.go` — B1, B5 fixes
- `internal/usecase/bundle.go` — B4, FR-031 clock
- `internal/usecase/search.go` — B3 distinction
- `internal/infra/config/config.go` — C1, C2, D7 validation
- `internal/infra/registry/yaml.go` — D7 flock
- `internal/infra/transport/server.go` — D1–D4 hardening
- `cmd/tahu/main.go` — C1, C2, C5 wiring
- `internal/domain/*.go` — FR-029–032
- `.golangci.yml` — FR-025
- `.github/workflows/ci.yml` — FR-026, FR-027
- `.github/workflows/cd.yml` — FR-026
- `AGENTS.md`, `documents/` — FR-017

### Files to delete (Phase 5)
- `internal/adapter/okf/indexer.go`

---

## Breaking Changes

| Area | Change | Migration |
|---|---|---|
| `config.EmbeddingModel` | Unknown value → startup error instead of silent BM25 | Set `embedding_model: bm25` or remove field from config |
| `serve --config` | Flag now respected; previously silently ignored | No action required; behavior improves |
| HTTP `--bind` non-loopback | Rejected at startup | Add explicit opt-in config when needed (future story) |
| `config.Load` | Invalid field values return error | Fix config files with invalid port/transport/log_level values |
| `BundleRepository.Put` / registry | flock held during save | Negligible latency impact; single-writer assumption still holds |

---

## Implementation Phases

### Phase 1 — Correctness that reaches agents today (High priority)
- B1: NaN guard (zero-vector skip + search guard)
- B4: ReindexBundle scope-delete before upsert
- B5: HNSWStore.Load dims validation + reset
- A3: WriteReserved mutex fix
- Ship with RED→GREEN tests for each

### Phase 2 — Boundary-guard consolidation (structural)
- FR-001: Bundle path resolver
- FR-002–007: Route all repo methods through resolver
- Traversal + symlink tests

### Phase 3 — Operational hardening
- FR-020–024: HTTP timeouts, non-loopback block, stdio shutdown, panic recovery, read caps
- FR-025–028: gosec, SHA-pin, integration CI, config validation, flock

### Phase 4 — Honesty & retrieval quality
- FR-015/016/019: EmbeddingModel error, --config, server version
- FR-012/013/014: vocab IDF, search distinction, scope boundary
- FR-017: doc reconciliation

### Phase 5 — Test uplift & domain hardening
- FR-029–032: constructors, OKFFrontmatter.Validate, clock injection, domaintest
- FR-018: delete indexer.go
- Outer-layer test coverage to floors
- Integration test
- CI gate ratchet

---

## Success Criteria and Acceptance Criteria

### Correctness
- [ ] `TestHNSWStore_ZeroVector_NoNaNScores` fails before fix, passes after
- [ ] `TestReindexBundle_RemovesStaleChunks` fails before fix, passes after
- [ ] `TestHNSWStore_Load_ValidatesDims` fails when persisted dims differ
- [ ] `go test -race ./...` passes clean after every phase

### Security / boundary guards
- [ ] `TestFileNodeRepository_ReadReserved_RejectsTraversal` fails before FR-002, passes after
- [ ] `TestFileNodeRepository_WriteReserved_RejectsTraversal` same for writes
- [ ] Symlink inside bundle root rejected with `ErrPathEscape` on read

### Config / advertised capabilities
- [ ] `TAHU_EMBED_MODEL=minilm-l6-v2 tahu serve` exits non-zero with clear error
- [ ] `tahu serve --config /tmp/custom.yaml` reads that file, not `~/.tahu/config.yaml`
- [ ] `TestConfig_EnvOverridePrecedence` passes
- [ ] `TestConfig_InvalidPort_ReturnsError` passes

### Operational hardening
- [ ] `TestServeHTTP_Healthz` passes (200 + 405)
- [ ] `TestLoggingMiddleware_PopulatesRequestID` passes
- [ ] HTTP server sets `ReadHeaderTimeout` ≥ 5s
- [ ] `tahu serve --transport http --bind 0.0.0.0` fails at startup with clear error
- [ ] Handler panic recovered, logged, returned as error CallToolResult

### Documentation / dead code
- [ ] `AGENTS.md` contains no references to Node/Edge/Facet/Graph/GraphRepository
- [ ] `indexer.go` does not exist (deleted in Phase 5)
- [ ] `internal/domain/domaintest/` exists and is imported by test packages

### Test coverage
- [ ] `TestYAMLBundleRepository_RoundTrip` passes
- [ ] `TestHNSWStore_Delete_RemovesChunk` passes
- [ ] `TestHNSWStore_AllOOV_Search_ReturnsEmpty` passes — a bundle where every document produces a zero-norm embedding returns an empty (non-error) search result
- [ ] At least one `//go:build integration` test exists and runs in CI
- [ ] Coverage floors met per NFRs

---

## Risks and Mitigation

| Risk | Mitigation |
|---|---|
| Vocab-selection change alters search results | Ship with fixed evaluation set; measure delta before merge |
| Path resolver touches every filesystem call | Full existing test suite + new traversal tests gate the change |
| flock syscall not available on all platforms | Scope to Linux/macOS (Windows deferred per NG8); gate behind build tag if needed |
| Deleting indexer.go removes current coverage | Phase 5 new tests replace coverage before deletion |
| HTTP bind block may affect existing setups | Loopback-only default; block is only on non-loopback; no stdio impact |

---

## Timeline and Milestones

| Phase | Scope | Gate |
|---|---|---|
| Phase 1 | NaN, stale reindex, dims, mutex | `go test -race ./...` green; RED→GREEN tests present |
| Phase 2 | Path resolver + all repo methods | Traversal tests pass; no regression in existing suite |
| Phase 3 | HTTP/stdio hardening + CI | gosec clean; SHA-pinned; integration CI job green |
| Phase 4 | Config honesty + retrieval quality | EmbeddingModel error on unknown; vocab eval set measured |
| Phase 5 | Test uplift + domain + indexer deletion | Coverage floors met; integration test present; all lints clean |

---

## References

- Source PRD: `specs/260705-code-and-design-quality-uplift/code-and-design-quality-uplift-PRD.md`
- RFC: `RFC-001-code-and-design-quality.md`
- Architecture decisions: `documents/arch-decisions-record.md`
- Agent conventions: `AGENTS.md`
