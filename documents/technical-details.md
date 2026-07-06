# Technical Details

> Technical deep-dive: architecture, data flows, interfaces, embedding, configuration. Update whenever the architecture, data model, or configuration surface changes.

## Architecture — Clean Architecture layer diagram

Dependency rule: inner layers never import outer layers.

```
┌──────────────────────────────────────────────────────────────────┐
│  cmd/tahu/                                                        │
│  Entry point: load config, wire DI graph, start transport server  │
└───────────────────────────┬──────────────────────────────────────┘
                            │ imports
┌───────────────────────────▼──────────────────────────────────────┐
│  internal/infra/                                                  │
│  config/   — YAML config loader + env-var overlay                 │
│  registry/ — YAMLBundleRepository (BundleRepository impl)        │
│  transport/— stdio MCP server + HTTP/SSE MCP server wiring       │
└───────────────────────────┬──────────────────────────────────────┘
                            │ imports
┌───────────────────────────▼──────────────────────────────────────┐
│  internal/adapter/                                                │
│  mcp/         — 14 MCP tool handlers (thin: delegate to usecase) │
│  okf/         — OKF parser, linker, validator                    │
│               — BundlePathResolver (single path-resolution gateway)│
│               — FileNodeRepository (NodeRepository impl)         │
│  embedder/    — BM25Embedder (Embedder impl, pure-Go, IDF-sorted)│
│               — chunker (splits OKFConcept into EmbeddingChunks) │
│  vectorstore/ — HNSWStore (VectorStore impl, coder/hnsw)         │
│  llm/         — (reserved; no implementations in v0.1)           │
└───────────────────────────┬──────────────────────────────────────┘
                            │ imports
┌───────────────────────────▼──────────────────────────────────────┐
│  internal/usecase/                                                │
│  BundleService  — bundle registration and reindex; clock-injected│
│  ConceptService — concept CRUD/list/link/type-list; clock-injected│
│  SearchService  — SemanticSearch, KeywordSearch, RAGSearch;      │
│                   separate Embedder+KeywordEmbedder fields        │
└───────────────────────────┬──────────────────────────────────────┘
                            │ imports (interfaces only)
┌───────────────────────────▼──────────────────────────────────────┐
│  internal/domain/                                                 │
│  Types: BundleEntry, OKFConcept, OKFFrontmatter, ConceptRef,     │
│         ConceptLink, EmbeddingChunk, ScoredChunk, Scope          │
│  Interfaces: Embedder, VectorStore, NodeRepository,              │
│              BundleRepository                                     │
│  Errors: ErrNotFound, ErrConflict, ErrValidation, ...            │
│  Zero external dependencies — stdlib only                        │
└──────────────────────────────────────────────────────────────────┘
```

## Key domain interfaces

```go
// Embedder converts text slices into dense vector representations.
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dims() int
}

// VectorStore persists and queries embedding chunks.
type VectorStore interface {
    Upsert(ctx context.Context, chunks []EmbeddingChunk) error
    Search(ctx context.Context, query []float32, scope Scope, topK int) ([]ScoredChunk, error)
    Delete(ctx context.Context, ids []string) error
    Persist(ctx context.Context) error
    Load(ctx context.Context) error
}

// NodeRepository provides read/write access to OKF concept documents
// and reserved OKF files (index.md, log.md).
type NodeRepository interface {
    Get(ctx context.Context, ref ConceptRef) (*OKFConcept, error)
    Put(ctx context.Context, ref ConceptRef, concept *OKFConcept) error
    List(ctx context.Context, bundleAlias string, subPath string) ([]ConceptRef, error)
    ListTypes(ctx context.Context, bundleAlias string) ([]string, error)
    ReadReserved(ctx context.Context, bundleAlias string, relPath string) (string, error)
    WriteReserved(ctx context.Context, bundleAlias string, relPath string, content string) error
}

// BundleRepository provides CRUD access to registered bundle metadata.
type BundleRepository interface {
    Get(ctx context.Context, alias string) (*BundleEntry, error)
    Put(ctx context.Context, entry BundleEntry) error
    Delete(ctx context.Context, alias string) error
    List(ctx context.Context) ([]BundleEntry, error)
}
```

## Data flow — concept_write

```
Agent → MCP tool: concept_write(ref, type, title, body, ...)
  adapter/mcp: validate ref format, validate body ≤ 1 MB
  → usecase.ConceptService.WriteConcept(ctx, ref, concept)
    → domain.NodeRepository.Put(ctx, ref, concept)
      → adapter/okf: validate OKF invariants (type required, reserved names blocked)
      → write <bundle-root>/<rel-path>.md to disk (frontmatter + body)
      → regenerate index.md in affected directory
      → append timestamped entry to log.md in affected directory
  ← "concept written: alias:path"
← MCP text response
```

## Data flow — search_rag

```
Agent → MCP tool: search_rag(query, scope, top_k, min_score)
  adapter/mcp: validate query ≤ 4 KB, parse scope string
  → usecase.SearchService.RAGSearch(ctx, query, scope, topK, minScore)
    → domain.Embedder.Embed(ctx, [query])   // BM25 — pure-Go, in-process
    → domain.VectorStore.Search(ctx, vec, scope, topK)
      → adapter/vectorstore: HNSW ANN search, scope filter, score filter (≥ minScore)
  ← []ScoredChunk  (up to top_k, score ≥ min_score, descending order)
← MCP JSON response
```

## Embedding tier (v0.1)

| Tier | Config value | CGo | Status |
|---|---|---|---|
| Pure-Go BM25 | `bm25` | No | Implemented; default in v0.1 |
| ONNX MiniLM-L6-v2 | `minilm-l6-v2` | Yes | Accepted config value; implementation deferred (ADR-006) |

The `domain.Embedder` interface is identical for both tiers. Switching is a config-file change (no code change). BM25 vectors are 4096-dimensional sparse keyword representations.

## Vector index

- **Algorithm:** HNSW (Hierarchical Navigable Small World) via `github.com/coder/hnsw`
- **Storage:** `~/.tahu/hnsw.index` (single shared file, alongside `registry.yaml`)
- **Update strategy:** incremental on write; full rebuild via `bundle_reindex` tool or `tahu bundle reindex` CLI
- **Parameters:** `hnsw_ef_construction` (build-time search depth, default 200), `hnsw_m` (max connections per node, default 16)
- **Startup:** loaded from disk on `serve`; cold-start (no file) is a no-op
- **Shutdown:** flushed to disk on clean SIGINT/SIGTERM

## MCP transport

| Mode | Config / flag | Protocol |
|---|---|---|
| stdio | `transport: stdio` / `--transport stdio` | JSON-RPC 2.0 over stdin/stdout |
| HTTP/SSE | `transport: http` / `--transport http` | HTTP POST + Server-Sent Events |

Both modes expose the same 14-tool surface.

## OKF write invariants enforced by the daemon

1. The `type` frontmatter field must be present and non-empty.
2. Writing to `index.md` or `log.md` via `concept_write` is rejected.
3. After any write: `index.md` is regenerated for the affected directory.
4. After any write: a timestamped entry is appended to `log.md`.
5. Frontmatter keys are written in canonical order: `type, title, description, resource, tags, timestamp`.
6. Concept body must not exceed 1 MB.

## OKF format primer

An OKF bundle is a directory tree of UTF-8 markdown files, each with a YAML frontmatter block:

```markdown
---
type: runbook
title: Deploy Pipeline
description: Steps to deploy the payments service
tags: [devops, payments]
timestamp: 2026-07-05T12:00:00Z
---

# Deploy Pipeline

See also [Payments API](../apis/payments-api.md).
```

Reserved files per directory:
- `index.md` — auto-generated directory listing (written by daemon on every concept write)
- `log.md` — append-only change log (written by daemon on every concept write)

Links between concepts use standard markdown hyperlinks with relative paths.

## Performance targets

| Metric | Target |
|---|---|
| Startup (10 bundles) | < 500 ms |
| Semantic search p99 (10k concepts) | < 200 ms |
| Memory (10k concepts) | < 512 MB RSS |
| Write + reindex single concept | < 100 ms |

## Configuration reference

Config file: `~/.tahu/config.yaml`. Missing file is treated as an empty file; all keys fall back to defaults. Environment variables overlay the file. CLI flags overlay environment variables.

| YAML key | Env var | Default | Description |
|---|---|---|---|
| `transport` | `TAHU_TRANSPORT` | `stdio` | MCP transport: `stdio` or `http` |
| `port` | `TAHU_PORT` | `3000` | TCP port for HTTP transport |
| `bind_addr` | — | `127.0.0.1` | Bind address for HTTP transport |
| `bundle_registry` | `TAHU_REGISTRY` | `~/.tahu/registry.yaml` | Path to bundle registry YAML file |
| `embedding_model` | `TAHU_EMBED_MODEL` | `bm25` | Embedding backend: `bm25` or `minilm-l6-v2`* |
| `embedding_batch_size` | — | `64` | Texts embedded per batch |
| `hnsw_ef_construction` | — | `200` | HNSW build-time search depth |
| `hnsw_m` | — | `16` | HNSW max connections per node |
| `log_level` | `TAHU_LOG_LEVEL` | `info` | Slog verbosity: `debug`, `info`, `warn`, `error` |

*`minilm-l6-v2` is accepted as a config value but not implemented in v0.1 (ADR-006).

---

*For architectural decisions see [`arch-decisions-record.md`](arch-decisions-record.md).*
