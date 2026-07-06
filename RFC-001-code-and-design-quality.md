# RFC-001: Code & Design Quality Uplift for `tahu`

**Status:** Proposed
**Date:** 2026-07-05
**Author:** Architecture review (senior solution architect / principal engineer pass)
**Reviewers:** _pending — core maintainers_
**Affects:** `internal/adapter/*`, `internal/usecase/*`, `internal/infra/*`, `internal/domain/*`, `cmd/tahu`, CI/CD, docs
**Supersedes / relates to:** feeds the ADR log in `documents/arch-decisions-record.md`

---

## 1. Summary

`tahu` is a well-structured codebase. Clean Architecture layering is respected (verified: `domain` imports only stdlib; `usecase` imports only stdlib + `domain`), error wrapping with `%w` is consistent, the HNSW persistence path uses correct atomic temp-file+rename, and much of the test suite is genuinely table-driven and spec-referenced. This is a solid v0.1 foundation.

This RFC is about making it **evolvable and trustworthy** as it grows. A layer-by-layer review surfaced ~30 findings that cluster into **four themes**. None are Critical — nothing is remotely exploitable in normal single-tenant local operation, and no path destroys data. But several are **High**: they either return wrong results to agents today, or they are latent defects one refactor away from becoming security or correctness incidents.

Two findings were reproduced empirically during the review (not merely asserted):

- **Zero-vector embeddings produce `NaN` similarity scores** that corrupt search ranking. Confirmed with a live test: a query with no in-vocabulary terms makes *every* result score `NaN`.
- **`config.EmbeddingModel` is a silent no-op.** It is read from config/env but never consumed by the wiring; the BM25 embedder is hard-wired regardless. Confirmed by grep — the field appears only in `config.go`, never in `cmd/tahu`.

The unifying architectural narrative is in Theme A: **invariants are guarded at one layer instead of at the boundary that owns them.** Fixing that theme structurally (not case-by-case) is the single highest-leverage change in this RFC.

---

## 2. Goals & Non-Goals

**Goals**
- Eliminate the correctness defects that return wrong search results to agents.
- Move security/containment invariants to the boundary that owns them, so future callers can't bypass them.
- Close the "advertised but not implemented" gap between docs/config and running behavior.
- Raise test coverage where it is lowest and highest-risk (the outer layers) and add the integration tests the conventions already promise.

**Non-Goals**
- Replacing BM25 with a true dense embedder (ONNX/MiniLM). That is a product decision tracked separately; this RFC only makes the *seam* honest and correct.
- Windows support (already deferred, NG8).
- Multi-tenant / networked hardening beyond making HTTP mode safe-by-default and clearly documented.

---

## 3. Themes

### Theme A — Invariants guarded at one layer, not at the boundary that owns them

This is the throughline. Several guards are enforced in exactly one caller; the component that owns the invariant trusts its callers instead of enforcing it. Each is safe *today* only because the one current caller happens to do the right thing — which is precisely the fragility we want to remove.

- **A1 (High, latent security).** `FileNodeRepository.ReadReserved` / `WriteReserved` (`internal/adapter/okf/repository.go:215,234`) build the target with only `filepath.Join(root, filepath.Clean(relPath))`. `filepath.Clean` does **not** neutralize `..` traversal after a join. `Get`/`Put` self-validate via `ValidateConceptPath`; these two do not. It is not exploitable today only because the MCP handlers (`handlers.go:253,273`) pre-reject `..` on `dir_path`. Any new caller (a CLI command, a new tool, a future use case) that forgets that pre-check reintroduces arbitrary file read/write. **Fix:** enforce `ValidateConceptPath`-style containment *inside* the repository methods (and `List`), so the filesystem trust boundary is the repository, not the handler.

- **A2 (Medium, security).** `WriteConcept`'s input-size and path-escape checks (`domain.ErrInputTooLarge`, `domain.ErrPathEscape`) live only at the adapter boundary. `ConceptService.WriteConcept` is directly callable and applies neither. **Fix:** re-validate `ref.RelativePath` (non-empty, `.md`, no `..`) in the use case as defense-in-depth, or route all writes through a single validating entry point.

- **A3 (Medium, concurrency correctness).** `WriteReserved` never takes `f.mu`, though the field is documented as "serializes writes across all bundles" and `Put` holds it (`repository.go:27,107,228`). Cross-method write serialization is therefore broken at the repository level; it only works because `ConceptService` adds a *separate* per-bundle advisory lock. **Fix:** acquire `f.mu` in `WriteReserved` so the repository's own stated invariant holds.

- **A4 (Medium, robustness).** `ValidatePath` (`adapter/mcp/validation.go:33`) is a weaker parallel guard than `ValidateConceptPath`: it rejects only exact `..` components, not absolute paths, `.` segments, or symlink escapes, and it is skipped entirely when the sub-path is empty. **Fix:** collapse to one canonical path-validation routine used everywhere, rather than two implementations of subtly different strength.

- **A5 (Medium, security).** Symlink traversal: `ValidateConceptPath` (`validator.go:23`) canonicalizes the bundle *root* via `EvalSymlinks` but never the final target, then does a lexical prefix check. A symlink living under the root but pointing to `/etc/passwd` passes, and `os.ReadFile` follows it. **Fix:** `EvalSymlinks` (or `Lstat`-and-reject) the resolved target before reading.

> **Structural resolution for Theme A:** introduce a single "bundle path resolver" that every filesystem access goes through, returning a validated absolute path or an error. Make it the *only* way to turn `(bundleAlias, relPath)` into a real path. This retires A1, A2, A4, A5 as a class rather than one PR at a time.

### Theme B — Retrieval correctness & the "semantic" gap

The product is named and marketed as *semantic* search, but the retrieval layer has correctness bugs and a capability gap that will erode trust as the corpus grows.

- **B1 (High, correctness — reproduced).** Zero-vector embeddings → `NaN` scores. `BM25Embedder.Embed` returns an all-zeros vector for any text with no in-vocabulary terms (`bm25.go:172`). `coder/hnsw`'s `CosineDistance` returns `NaN` for a zero vector, and `Search` computes `score = 1 - NaN = NaN` (`hnsw.go:145`). `NaN` then flows into `sort.SliceStable` (`hnsw.go:151`), whose `>` comparator is always false for `NaN`, giving undefined ordering. **Reproduced:** an all-zero indexed chunk returns `score=NaN`; a zero-vector *query* makes every result `NaN`. **Fix:** skip indexing zero-norm vectors, short-circuit empty/zero queries to return no results, and sanitize/guard `NaN` before sorting. Ship the repro as a RED test.

- **B2 (High, retrieval quality — bites at scale).** `buildVocab` (`bm25.go:140`) caps the vocabulary at the `maxDims` (4096) terms of **highest document frequency**, breaking ties alphabetically. Highest-DF terms are the *least* discriminative; the rare, high-IDF terms that retrieval most depends on are the first discarded once a corpus exceeds 4096 distinct tokens. Small bundles never hit the cap and look fine, which is exactly why this is dangerous — it degrades silently as the KB grows. **Fix:** select vocabulary by IDF/discriminativeness (or raise/remove the cap with sparse storage), and document the retrieval model honestly.

- **B3 (Medium, correctness / honesty).** `SemanticSearch` and `KeywordSearch` are byte-for-byte identical (`search.go:28,52`) and operate on the same single `Embedder` field. The doc comment claims the distinction is "which Embedder is wired in," but on any given instance both do the same thing — FR-013 "keyword search" is not actually a distinct capability. **Fix:** either give `SearchService` two embedders (dense + keyword) or collapse to one method with a mode, and delete the misleading comment.

- **B4 (High, correctness).** `ReindexBundle` (`bundle.go:150`) only `Upsert`s current concepts; it never `Delete`s. A "full reindex" therefore *accumulates* stale vectors — deleted/renamed concepts, and orphaned `:1`,`:2`… chunks from documents that now produce fewer chunks, keep surfacing in results forever. **Fix:** scope-delete the bundle's existing chunks (or diff against the listed refs) as part of reindex.

- **B5 (Medium, correctness).** `HNSWStore.Load` (`hnsw.go:298`) imports the graph without validating persisted dimensionality against configured `dims`, and `readMeta` decodes into the *existing* `chunks` map (merge, not replace). A stale index of a different dimension loads silently; a double `Load` yields inconsistent graph/metadata. **Fix:** validate dims on import; reset graph+chunks at the start of `Load`.

- **B6 (Low/Medium, correctness).** `ScopePath` filtering uses a raw `strings.HasPrefix(c.ConceptPath, scope.SubPath)` (`hnsw.go:340`), so scope `path:kb:foo` also matches `foobar/…`. `ParseScope` additionally accepts an empty sub-path (`"path:kb:"`) as a silent bundle-scope alias. **Fix:** add a path-separator boundary to the prefix check; reject empty sub-path with a wrapped sentinel error.

### Theme C — Advertised vs. real (config no-ops, doc drift, dead code)

A cluster of places where the code *claims* a capability it does not deliver. Individually minor; together they erode the trust a maintainer places in the docs and config, which is what makes a codebase safe to change.

- **C1 (Medium — reproduced).** `config.EmbeddingModel` (`config.go:29`) is read from file/env/default but **never consumed** in `buildServices`; BM25 is hard-wired. `embedding_model: minilm-l6-v2` / `TAHU_EMBED_MODEL` silently does nothing. **Fix:** either honor it (select embedder by value, error on unknown) or remove the field until ONNX lands. `EmbeddingBatchSize` is likewise unused.
- **C2 (Medium).** `serve --config <path>` is accepted then discarded (`main.go:97`, `_ = configPath`); `config.Load()` always reads the hardcoded `~/.tahu/config.yaml`. **Fix:** thread the path into `config.Load(path)` or remove the flag.
- **C3 (Medium, doc drift).** `AGENTS.md` documents a core domain model of `Node`/`Edge`/`Facet`/`Graph` and a `GraphRepository` "with semantic search over nodes." **None of these exist.** The real model is Concept-based (`OKFConcept`, `BundleEntry`, `EmbeddingChunk`, `ConceptRef`); there is no `GraphRepository`. New contributors will be actively misled. **Fix:** reconcile `AGENTS.md` and `documents/` with the real model.
- **C4 (Medium, maintainability).** `internal/adapter/okf/indexer.go` (`GenerateIndex`, `AppendLog`) is **dead code** — production index/log generation lives in `usecase/concept.go` (`regenerateIndex`/`appendLog`). The dead file carries its *own* non-atomic `os.WriteFile`, markdown-injection, and unbounded-read issues, i.e. a second, divergent implementation of the same feature that an auditor must still reason about. **Fix:** delete it (and its tests), or make the use case delegate to it — one source of truth.
- **C5 (Low).** MCP server version is hardcoded `"0.1.0"` (`transport/server.go:24`), diverging from the ldflags-injected `main.version`. **Fix:** thread the real version through.

### Theme D — Operational hardening & outer-layer test coverage

The outer layers (`infra`, `cmd`) — the layer the conventions explicitly call out as where all the risky I/O and third-party SDKs live — are both the least hardened and the least tested.

- **D1 (High, DoS).** The HTTP server sets no `ReadTimeout`/`ReadHeaderTimeout`/`WriteTimeout`/`IdleTimeout` and no `MaxHeaderBytes` (`transport/server.go:65`). Classic Slowloris exposure (gosec G112/G114). **Fix:** set header/read/idle timeouts; scope the long-lived-write exception to the SSE route.
- **D2 (High, access control).** HTTP `/sse` and `/message` have no auth. Default bind is `127.0.0.1` (good), but `--bind 0.0.0.0` exposes all 14 tools — including filesystem writes — to any network client. **Fix:** require a bearer/shared secret in HTTP mode and/or refuse non-loopback binds without explicit opt-in; document loudly.
- **D3 (Medium, availability).** stdio shutdown ignores context (`ServeStdio(_ context.Context …)`, `server.go:34`). `main` installs `signal.NotifyContext` and a `defer store.Persist()`, but nothing unblocks the stdio serve on signal, and `NotifyContext` also suppresses default SIGINT termination — so Ctrl-C can hang the process and the vector index is never persisted. **Fix:** select on `ctx.Done()` in `ServeStdio`, or drop the misleading param and document EOF-only shutdown.
- **D4 (Medium, availability).** No `recover()` in the tool-handler middleware (`server.go:114`). In stdio mode a panic in any handler crashes the daemon. **Fix:** deferred `recover()` that logs and returns an error `CallToolResult`.
- **D5 (Medium, DoS).** Unbounded `os.ReadFile` on the read path (`repository.go:61`, `:189`; `indexer.go:43`). The 1 MB cap is enforced only on MCP *writes*; any large file already on disk is read fully into memory, and `ListTypes` reads+parses every file in a bundle. **Fix:** stat-and-reject or `io.LimitReader` before reading; add a per-request body cap via `http.MaxBytesReader`.
- **D6 (Medium, supply chain / CI).** `gosec` is not in `.golangci.yml`, so D1-class issues are never flagged. GitHub Actions are tag-pinned (`@v4`, `@v5`, `@v7`, `@v2`), not SHA-pinned, while CD holds `contents: write`. Integration tests (`-tags integration`, per the Makefile) are never run in CI. **Fix:** add `gosec`; pin actions to commit SHAs; run integration tests in CI.
- **D7 (Low).** Cross-process registry writes are load-then-save under an in-process `RWMutex` only (`registry/yaml.go`); two `tahu` processes can lose updates (atomic rename prevents torn files, not lost updates). Config values are accepted without validation (bad `TAHU_PORT` silently ignored; invalid `transport`/`log_level` silently fall back). **Fix:** advisory file lock around load→save (or document single-writer); validate config and fail loudly.
- **D8 (Low, content injection).** Generated `index.md`/`log.md` do not escape `|`/newline in filenames, titles, or log entries (`concept.go:179`, and the dead `indexer.go`). A crafted title injects fake table rows. Content-only. **Fix:** escape pipe/newline when composing tables and log lines.

---

## 4. Testing — required as a first-class deliverable

The user asked specifically for missing/weak tests. Measured coverage (`go test -cover ./...`; suite passes clean under `-race`):

| Package | Coverage | Note |
|---|---|---|
| `internal/domain` | 100.0% | strong |
| `internal/usecase` | 95.9% | strong (real error-path injection) |
| `internal/adapter/embedder` | 87.5% | ok |
| `internal/adapter/vectorstore` | 74.1% | `Delete` at 0% |
| `internal/adapter/mcp` | 61.4% | happy paths of read/write untested |
| `internal/adapter/okf` | 45.0% | real `List`/`ReadReserved`/`WriteReserved` at 0% |
| `internal/infra/transport` | 4.5% | middleware + `/healthz` + ServeHTTP untested |
| `internal/infra/config` | **0.0%** | no test file |
| `internal/infra/registry` | **0.0%** | no test file — the only real `BundleRepository` impl |
| `cmd/tahu` | 0.0% | `parseConceptRef` is pure logic, untested |

**Headline gaps**

1. **The outer layers — the ones the conventions call highest-risk — are near-zero.** `infra/config` and `infra/registry` have *no test files at all*. A registry round-trip test (`Put` → save → load → `Get` returns equal) is the single highest-value missing test in the repo.
2. **No integration tests exist**, despite `AGENTS.md` promising `//go:build integration` tests in `internal/adapter/*_integration_test.go`. This is a documented-but-unmet convention.
3. **Coverage-padding assertions.** `handlers_coverage_test.go` happy-path tests assert only `!result.IsError` / `result != nil` — never the `Content`. `TestHandleBundleList_ReturnsBundleEntries` seeds a bundle but never checks it appears. `usecase/coverage_test.go` asserts `err != nil` without `errors.Is` on the sentinel — a swallowed-and-replaced error would pass. **Tighten to assert the value/typed error, not merely that an error occurred.**
4. **Concurrency is tested only against fakes**, not the real `FileNodeRepository.mu` or `HNSWStore.mu`.
5. **Dead/duplicated fakes.** `internal/domain/fake_test.go` (300 lines) is referenced by no domain test and can't be imported by other packages, so `usecase`/`mcp` re-declare near-identical fakes. Promote to `internal/domain/domaintest/` and dedupe.

**Concrete tests to add (each also serves as the RED test for a fix above)**

- `TestHNSWStore_ZeroVector_NoNaNScores` (B1) — the reproduced bug; must fail before fix.
- `TestReindexBundle_RemovesStaleChunks` (B4).
- `TestYAMLBundleRepository_RoundTrip` + `_ConcurrentPut` (D7, coverage).
- `TestFileNodeRepository_ReadReserved_RejectsTraversal` (A1) — fails today.
- `TestConfig_EnvOverridePrecedence` + `_MalformedPortIgnored` (C1/D7).
- `TestServeHTTP_Healthz` + `TestLoggingMiddleware_PopulatesRequestID` (D-coverage, FR-021 is asserted nowhere).
- `TestFileNodeRepository_List_*`, `ListTypes_Deduplicates`, `WriteReserved_RoundTrip` (okf 45%→).
- `TestHNSWStore_Delete_RemovesChunk` (vectorstore `Delete` 0%).
- `TestChunkConcept_MultibyteRunes` (unicode boundary), malformed-frontmatter round-trip, `parseConceptRef` table test.
- One real integration test under `//go:build integration` to make the convention true.

**Suggested CI gate additions:** extend the existing domain/usecase ≥90% gates to a `infra`/`adapter` floor (start at current+guardrail, ratchet up), and run `-tags integration` in a dedicated job.

---

## 5. Domain modeling (cross-cutting, Medium)

The model is somewhat **anemic**: value objects (`ConceptRef`, `BundleEntry`, `EmbeddingChunk`, `Scope`) have no validating constructors and can be built in invalid states. The one real invariant — "frontmatter `type` is required" — is an ad-hoc check in `WriteConcept`, not a domain method. `ConceptRef.String()` concatenates `alias + ":" + path` with nothing rejecting a `:` inside either field, which makes the `alias:path:chunkIndex` chunk-ID grammar ambiguous. **Fix (incremental):** add `NewConceptRef`/`Validate` constructors enforcing non-empty/`.md`/no-`:`, and make the `type`-required rule a method on `OKFFrontmatter`. Inject a `Clock` (`func() time.Time`) so timestamps in `AddBundle`/`appendLog` are testable. These are not urgent but pay off exactly as the model grows — which is the point of this RFC.

---

## 6. Proposed roadmap (prioritized workstreams)

Sequenced so each phase is independently shippable and TDD-friendly (write the RED test named in §4 first).

**Phase 1 — Correctness that reaches agents today (High).**
B1 (NaN), B4 (stale reindex), B5 (Load dims/reset), A3 (WriteReserved mutex). Small, high-value, each with a repro test.

**Phase 2 — Boundary-guard consolidation (Theme A, structural).**
Introduce the single bundle path resolver; route `Get`/`Put`/`List`/`ReadReserved`/`WriteReserved` through it; retire A1/A2/A4/A5 as a class. Add the traversal + symlink tests.

**Phase 3 — Operational hardening (Theme D).**
D1 (timeouts), D2 (HTTP auth + bind policy), D3 (stdio shutdown), D4 (panic recovery), D5 (read caps), D6 (gosec + SHA-pin + integration in CI).

**Phase 4 — Honesty & retrieval quality (Themes B/C).**
C1/C2 (honor or remove `EmbeddingModel`/`--config`), C3 (doc reconciliation), C4 (delete dead `indexer.go`), B2 (vocab selection), B3 (semantic vs keyword), B6 (scope boundary).

**Phase 5 — Test uplift & domain hardening (§4, §5).**
Outer-layer tests to a coverage floor, integration test, fake dedupe, validating constructors, injected clock. Ratchet CI gates.

_Rule of engagement (per `AGENTS.md`): each phase is SDD+TDD — update/write the spec, write the failing test, then the fix. Architecturally significant items (path resolver, HTTP auth, embedder selection, removing `GraphRepository` from docs) each get an ADR entry in `documents/arch-decisions-record.md`._

---

## 7. Risks & trade-offs of *doing* this work

- **HTTP auth (D2)** may break existing local orchestration setups — gate behind config with a loopback-only default so stdio users are unaffected.
- **Vocab-selection change (B2)** alters search results; land it with a fixed evaluation set so quality change is measured, not guessed.
- **Deleting `indexer.go` (C4)** removes tests that currently contribute coverage — expected, since the code is dead; net auditability improves.
- **Consolidated path resolver (Theme A)** touches every filesystem call; do it behind the existing repository tests plus the new traversal tests to avoid regressions.

---

## Appendix — Full findings index

Severity ceiling is **High** (nothing Critical; the path-traversal surface is defended at the boundary today, and no path destroys data). `R` = reproduced during review.

| ID | Sev | Area | File:line | One-line |
|---|---|---|---|---|
| A1 | High | Security (latent) | okf/repository.go:215,234 | Reserved-file ops lack containment; rely on caller |
| A2 | Med | Security | usecase/concept.go:116 | WriteConcept re-validates neither size nor path |
| A3 | Med | Concurrency | okf/repository.go:228 | WriteReserved bypasses repo write mutex |
| A4 | Med | Robustness | mcp/validation.go:33 | Weaker parallel path check; skipped when empty |
| A5 | Med | Security | okf/validator.go:23 | Symlink target not canonicalized before read |
| B1 | High `R` | Correctness | embedder/bm25.go:172 + vectorstore/hnsw.go:145 | Zero vector → NaN score → corrupt ranking |
| B2 | High | Retrieval@scale | embedder/bm25.go:140 | Vocab capped by highest-DF drops discriminative terms |
| B3 | Med | Correctness | usecase/search.go:28,52 | Semantic == Keyword; FR-013 not a real capability |
| B4 | High | Correctness | usecase/bundle.go:150 | Reindex never deletes stale vectors |
| B5 | Med | Correctness | vectorstore/hnsw.go:298 | Load skips dims check; merges instead of resets |
| B6 | Low/Med | Correctness | vectorstore/hnsw.go:340 | Scope prefix has no path boundary; empty subpath |
| C1 | Med `R` | Advertised≠real | config/config.go:29 | EmbeddingModel is a silent no-op |
| C2 | Med | Advertised≠real | cmd/tahu/main.go:97 | `--config` flag ignored |
| C3 | Med | Doc drift | AGENTS.md | Documents Node/Edge/Facet/Graph model that doesn't exist |
| C4 | Med | Dead code | okf/indexer.go | Divergent duplicate of index/log generation |
| C5 | Low | Consistency | transport/server.go:24 | MCP version hardcoded, diverges from build version |
| D1 | High | DoS | transport/server.go:65 | No HTTP timeouts (Slowloris) |
| D2 | High | Access control | transport/server.go:46 | HTTP transport has no authentication |
| D3 | Med | Availability | transport/server.go:34 | stdio shutdown ignores ctx; index not persisted on signal |
| D4 | Med | Availability | transport/server.go:114 | No panic recovery in middleware |
| D5 | Med | DoS | okf/repository.go:61,189 | Unbounded file reads into memory |
| D6 | Med | Supply chain/CI | .golangci.yml / .github | No gosec; actions tag-pinned; integration tests not run |
| D7 | Low | Robustness | registry/yaml.go; config.go:83 | Cross-process lost updates; config unvalidated |
| D8 | Low | Injection | usecase/concept.go:179 | index/log markdown not escaped |
| M-dom | Med | Modeling | domain/* | Anemic model; no validating constructors; non-injectable clock |
| T-* | — | Testing | see §4 | Outer layers ~0%; no integration tests; padding assertions |

_Findings were verified through a layer-by-layer review of the source. B1 (NaN ranking) and C1 (unused `EmbeddingModel`) were independently reproduced with throwaway tests/grep, and B4 (stale-vector reindex) was confirmed by reading `ReindexBundle` directly, before inclusion._
