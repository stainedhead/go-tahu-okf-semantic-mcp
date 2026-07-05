# Feature Spec: go-tahu-okf-semantic-mcp

**Created:** 2026-07-05  
**Source PRD:** `specs/260705-go-tahu-okf-semantic-mcp/go-tahu-okf-semantic-mcp-PRD.md`  
**Status:** Draft

---

## Executive Summary

Build a knowledge-management daemon in Go (`tahu`) that manages one or more OKF (Open Knowledge Format) bundles — hierarchical directory trees of UTF-8 markdown files with YAML frontmatter. The daemon exposes 14 MCP tools over stdio (for CLI agents) and HTTP/SSE (for orchestration agents), enabling agents to read, write, navigate, and semantically search OKF knowledge bases with zero external service dependencies.

OKF v0.1 (Apache 2.0, Google Cloud, June 2026) is the sole document format. All search — vector similarity and BM25 keyword — runs in-process using compiled-in libraries. The binary ships with no required cloud account, Python, Node, or separate vector database.

---

## Problem Statement

AI agents (CLI and orchestration) need to discover, read, write, and semantically search OKF knowledge bases across one or more bundles with scope control (global / bundle / sub-path). No existing tool provides all of these capabilities in a single zero-external-dependency binary.

**Affected actors:** CLI agents (Claude Code, aider), orchestration pipelines (LangGraph, custom), human operators.

---

## Goals / Non-Goals

### Goals
| ID | Goal |
|---|---|
| G1 | Manage any number of named OKF bundles (add, remove, list, reindex) |
| G2 | Expose full OKF read/navigate surface via MCP tools |
| G3 | Semantic similarity + keyword search, zero external service deps |
| G4 | OKF structural correctness enforced on all writes |
| G5 | Single Go binary; BM25 tier requires only `go build` |
| G6 | Dual MCP transport: stdio (CLI) + HTTP/SSE (orchestration) |
| G7 | Vector indexes persist on disk; survive daemon restart |
| G8 | Cross-bundle RAG retrieval with scoped results and source attribution |

### Non-Goals
- NG1: Real-time collaborative editing / conflict resolution
- NG2: Web UI
- NG3: Cloud-hosted deployment
- NG4: Non-OKF formats (HTML, DOCX, PDF)
- NG5: Typed semantic edges (OKF uses prose links)
- NG6: LLM answer synthesis (retrieval only)
- NG7: Authentication / authorization (v0.1)
- NG8: Windows support (v0.1)
- NG9: Metrics / tracing SDKs (v0.1)

---

## User Requirements — Functional Requirements

### Bundle Management (G1)

**FR-001** — `bundle_add` registers an OKF bundle by filesystem path and alias.  
- Validates path exists and contains at least one `.md` file  
- Rejects duplicate alias and duplicate root_path under a different alias  
- Returns structured error on failure  

**FR-002** — `bundle_remove` unregisters a bundle by alias without deleting files.

**FR-003** — `bundle_list` returns alias, root_path, concept_count, last_indexed_at for every registered bundle.

**FR-004** — `bundle_reindex` forces a full re-embed and reindex of a bundle; updates last_indexed_at.

### OKF Read / Navigate (G2)

**FR-005** — `concept_read` returns parsed frontmatter fields + markdown body for a valid `alias:relative/path.md` ref. Returns structured not-found error for missing path.

**FR-006** — `concept_list` returns all non-reserved `.md` files at the given directory level. Returns empty list (not error) for non-existent directory.

**FR-007** — `concept_links` returns all outbound markdown hyperlink targets from a concept body. Broken links are included with `broken: true`.

**FR-008** — `index_read` returns raw content of `index.md` at the given directory level, or structured not-found.

**FR-009** — `log_read` returns raw content of `log.md` at the given directory level, or structured not-found.

**FR-010** — `concept_type_list` returns all distinct `type` frontmatter values in a bundle.

### OKF Write (G2, G4)

**FR-011** — `concept_write` creates or updates a concept document.  
- Rejects if frontmatter `type` field is missing or empty  
- Rejects if target path is `index.md` or `log.md` at any level  
- Normalizes frontmatter key order: `type, title, description, resource, tags, timestamp`  
- Preserves unknown frontmatter keys after the standard set [TBD: per OQ-2 resolution]  
- Regenerates `index.md` for the affected directory after successful write  
- Appends timestamped entry to `log.md` after successful write  

### Search (G3, G8)

**FR-012** — `search_semantic` returns a ranked list of chunks `{source, chunk_text, score}` via vector similarity. Respects scope (global / bundle / path). No network call at query time.

**FR-013** — `search_keyword` returns a ranked list of chunks via Okapi BM25. Same scope semantics and response shape as FR-012.

**FR-014** — `search_rag` accepts query, scope, `top_k` (default 5, max 20), `min_score` (default 0.0). Returns up to `top_k` chunks with `score >= min_score` in descending order, each with `{source, chunk_index, chunk_text, score}`. Returns empty list if no chunks meet threshold. No synthesized answer.

### Transport and CLI (G5, G6)

**FR-015** — All 14 MCP tools respond identically in `--transport stdio` and `--transport http` modes.

**FR-016** — HTTP transport binds to `127.0.0.1` by default; `--bind` flag overrides.

**FR-017** — HTTP mode exposes `GET /healthz` returning `200 OK` when ready.

**FR-018** — `tahu` CLI exposes: `serve`, `bundle list/add/reindex`, `search`, `concept read`.

### Security (G4)

**FR-019** — All user-supplied paths are canonicalized (`filepath.Clean` + `filepath.EvalSymlinks`) and validated to be within a registered bundle root. Paths escaping the root return a structured permission-denied error.

**FR-020** — All MCP tool inputs validated against JSON Schema at the adapter boundary. Concept body ≤ 1 MB; other string inputs ≤ 4 KB.

### Observability

**FR-021** — Structured JSON logs via stdlib `slog` with fields: `level`, `time`, `request_id`, `bundle`, `tool`, `duration_ms`, `error`. `request_id` generated per MCP tool invocation, propagated via context.

---

## Non-Functional Requirements

| NFR | Target |
|---|---|
| Zero external services at query time | Hard — all search in-process |
| BM25 tier binary | `go build` only; no CGo, no native libs |
| Startup time (10 bundles) | < 500 ms |
| Semantic search p99 (10k concepts) | < 200 ms |
| Memory (10k concepts, 512-dim) | < 512 MB RSS |
| Concurrency | Thread-safe reads; serialized writes per bundle |
| OKF conformance | All writes validated; no reserved filename violations |
| Index persistence | Survives restart; incremental update on write; graceful cold-start rebuild |

---

## System Architecture

Clean Architecture — dependency rule: inner layers never import outer layers.

```
cmd/tahu/                    Entry point — wire all layers, parse flags/config, start server
internal/
  domain/                    Entities, value objects, repository interfaces, domain errors
                             ZERO external dependencies (stdlib only)
  usecase/                   Application logic; depends only on domain interfaces
  adapter/
    mcp/                     MCP tool handlers (thin — delegate to usecase)
    embedder/                domain.Embedder implementations (ONNX, BM25)
    vectorstore/             domain.VectorStore implementation (HNSW, disk-backed)
    okf/                     OKF document parser, frontmatter codec, link extractor
    llm/                     domain.Extractor (future; stub for v0.1)
  infra/
    transport/               MCP stdio + HTTP/SSE server wiring
    config/                  Config loading (file → env → flags)
    registry/                Bundle registry persistence (YAML file)
pkg/                         Exportable utilities (OKF codec, chunk helpers)
```

**Key domain interfaces:**
- `Embedder` — `Embed(ctx, []string) ([][]float32, error)`
- `VectorStore` — `Upsert(ctx, []EmbeddingChunk) error` / `Search(ctx, []float32, Scope, int) ([]ScoredChunk, error)`
- `NodeRepository` — `Get / Put / List` over `ConceptRef`

---

## Scope of Changes

### Files to create

| Path | Description |
|---|---|
| `cmd/tahu/main.go` | Entry point, DI wiring |
| `internal/domain/bundle.go` | BundleEntry, BundleRegistry types |
| `internal/domain/concept.go` | OKFConcept, ConceptRef, OKFFrontmatter |
| `internal/domain/chunk.go` | EmbeddingChunk, ScoredChunk, Scope |
| `internal/domain/errors.go` | Sentinel domain errors |
| `internal/domain/interfaces.go` | Embedder, VectorStore, NodeRepository, BundleRepository |
| `internal/usecase/bundle.go` | Bundle management use cases |
| `internal/usecase/concept.go` | Read/write/navigate use cases |
| `internal/usecase/search.go` | Semantic, keyword, RAG use cases |
| `internal/adapter/okf/parser.go` | Frontmatter + body parser |
| `internal/adapter/okf/linker.go` | Markdown link extractor (goldmark) |
| `internal/adapter/okf/indexer.go` | index.md + log.md generator |
| `internal/adapter/embedder/bm25.go` | Pure-Go BM25 embedder |
| `internal/adapter/embedder/onnx.go` | ONNX MiniLM embedder (CGo) |
| `internal/adapter/vectorstore/hnsw.go` | HNSW disk-backed store (coder/hnsw) |
| `internal/adapter/mcp/tools.go` | 14 MCP tool registrations |
| `internal/adapter/mcp/handlers.go` | Handler functions (thin) |
| `internal/adapter/mcp/schema.go` | JSON Schema definitions for all tools |
| `internal/adapter/mcp/validation.go` | Input validation (path confinement, size caps) |
| `internal/infra/transport/stdio.go` | stdio MCP transport |
| `internal/infra/transport/http.go` | HTTP/SSE MCP transport + /healthz |
| `internal/infra/config/config.go` | Config struct + loader |
| `internal/infra/registry/yaml.go` | Bundle registry YAML persistence |
| `pkg/okfcodec/` | Exported OKF codec utilities |

### Dependencies to add to `go.mod`

| Package | Purpose |
|---|---|
| `github.com/coder/hnsw` | Pure-Go HNSW vector index |
| `gopkg.in/yaml.v3` | YAML frontmatter parsing |
| `github.com/yuin/goldmark` | Markdown AST for link extraction |
| MCP Go SDK | MCP protocol (TBD: official or community) |

### Breaking Changes
None — greenfield.

---

## Acceptance Criteria

| Goal | Criteria |
|---|---|
| G1 | `bundle_add` rejects non-existent path / no .md files / duplicate alias / duplicate path. `bundle_remove` unregisters without deleting. `bundle_list` includes alias, root_path, concept_count, last_indexed_at. `bundle_reindex` updates last_indexed_at. |
| G2 | `concept_read` returns frontmatter+body or not-found. `concept_list` returns non-reserved .md files or empty. `index_read`/`log_read` return content or not-found. `concept_links` includes broken links with `broken:true`. |
| G3 | `search_semantic` and `search_keyword` return ranked `{source, chunk_text, score}` chunks. Scope boundaries enforced. No network call during search. |
| G4 | `concept_write` rejects missing `type` and reserved-filename targets. Regenerates `index.md` and appends `log.md` on success. |
| G5 | `go build ./cmd/tahu` succeeds. BM25 mode runs on Linux (amd64) + macOS (arm64) with no additional install. |
| G6 | All 14 tools respond identically in stdio and HTTP/SSE modes. |
| G7 | After restart, `search_semantic` returns same results without re-embedding. Cold-start rebuilds index lazily. |
| G8 | `search_rag` respects `top_k`/`min_score`, returns `{source, chunk_index, chunk_text, score}`. Empty list when no chunks meet threshold. |

**Quality gates:**
- All tests pass with `go test -race ./...`
- `golangci-lint run ./...` reports no issues
- Domain + Use Case layer coverage ≥ 90%
- No direct imports from `infra/` or `adapter/` in `domain/` or `usecase/`

---

## Risks and Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| No stable Go MCP SDK | Medium | High | Evaluate `mark3labs/mcp-go` and `modelcontextprotocol/go-sdk`; select in research phase |
| `coder/hnsw` persistence API differs from expectation | Low | Medium | Spike HNSW persistence in research; fallback: serialize index manually |
| ONNX model embedding quality insufficient for OKF prose | Medium | Medium | OQ-1; benchmark in research phase; BM25 always available as fallback |
| CGo complicates cross-compilation | Low | Low | BM25 tier is always CGo-free; ONNX tier documented as requiring native lib |

---

## Timeline and Milestones

| Milestone | Content |
|---|---|
| M1 — Domain complete | All domain types, interfaces, errors; no external deps; 100% test coverage |
| M2 — Adapters complete | OKF parser, BM25 embedder, HNSW store, MCP tool stubs; integration tests pass |
| M3 — Use cases complete | All 12 use case functions; acceptance criteria G1–G8 passing |
| M4 — Transport complete | stdio + HTTP/SSE; `tahu` CLI; `go build` produces working binary |
| M5 — Quality gate | Race detector clean; lint clean; coverage ≥ 90% on domain+usecase |

---

## References

- Source PRD: `specs/260705-go-tahu-okf-semantic-mcp/go-tahu-okf-semantic-mcp-PRD.md`
- OKF SPEC: `github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md`
- HNSW: `github.com/coder/hnsw`
- YAML: `gopkg.in/yaml.v3`
- Markdown: `github.com/yuin/goldmark`
- MCP specification: `modelcontextprotocol.io`
