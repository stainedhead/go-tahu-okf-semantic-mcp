# Technical Details

> Technical deep-dive: data flows, protocol details, performance characteristics, configuration reference. Update whenever the architecture, data model, or configuration surface changes.

## Architecture overview

Clean Architecture — dependency rule: inner layers never import outer layers.

```
cmd/tahu/           Wire all layers; start server
internal/
  domain/           Entities, value objects, repository interfaces, domain errors
                    Zero external dependencies — stdlib only
  usecase/          Application logic; composed from domain interfaces
  adapter/
    mcp/            MCP tool handler functions (thin — delegate to usecase)
    embedder/       domain.Embedder implementations (ONNX, BM25)
    vectorstore/    domain.VectorStore implementation (HNSW on disk)
    llm/            domain.Extractor implementations (LangChain-compatible)
  infra/            Transport (stdio/HTTP MCP), config loading
pkg/                Exportable utilities — OKF codec, embedding helpers
```

## Key domain interfaces

```go
// domain/embedder.go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// domain/vectorstore.go
type VectorStore interface {
    Upsert(ctx context.Context, chunks []EmbeddingChunk) error
    Search(ctx context.Context, query []float32, scope Scope, topK int) ([]ScoredChunk, error)
}

// domain/repository.go
type NodeRepository interface {
    Get(ctx context.Context, ref ConceptRef) (*OKFConcept, error)
    Put(ctx context.Context, ref ConceptRef, concept *OKFConcept) error
    List(ctx context.Context, scope BundleScope) ([]ConceptRef, error)
}
```

## Data flow — semantic search

```
Agent → MCP tool: search_semantic(query, scope, topK)
  → usecase.Search.Semantic(ctx, query, scope, topK)
    → domain.Embedder.Embed(ctx, [query])          // in-process, no network
    → domain.VectorStore.Search(ctx, vec, scope, topK)  // disk-backed HNSW
  ← []ScoredChunk{source, text, score}
← MCP response
```

## Embedding tiers

| Tier | Config value | CGo | Notes |
|---|---|---|---|
| ONNX MiniLM-L6-v2 | `model: minilm-l6-v2` | Yes | ~22 MB model embedded via `go:embed`; requires `libonnxruntime` at link time |
| Pure-Go BM25 | `model: bm25` | No | Keyword index; no native deps; works on Alpine musl |

The `domain.Embedder` interface is identical for both tiers; swap via config.

## Vector index

- Algorithm: HNSW (Hierarchical Navigable Small World)
- Storage: `<bundle-root>/.tahu/vectors.bin` (mmap'd on read)
- Update strategy: incremental on write; full rebuild via `bundle_reindex`
- Parameters: `hnsw_ef_construction` (default 200), `hnsw_m` (default 16)

## MCP transport

| Mode | Flag | Protocol |
|---|---|---|
| stdio | `--transport stdio` | JSON-RPC 2.0 over stdin/stdout |
| HTTP/SSE | `--transport http` | HTTP POST + Server-Sent Events |

Both expose the same 14-tool surface.

## OKF write invariants enforced by the daemon

1. `type` frontmatter field must be present and non-empty.
2. Writing to `index.md` or `log.md` via `concept_write` is rejected.
3. After any write: `index.md` is regenerated for the affected directory.
4. After any write: a timestamped entry is appended to `log.md`.
5. Frontmatter keys are normalized to canonical order: `type, title, description, resource, tags, timestamp`.

## Performance targets

| Metric | Target |
|---|---|
| Startup (10 bundles) | < 500 ms |
| Semantic search p99 (10k concepts) | < 200 ms |
| Memory (10k concepts, 512-dim) | < 512 MB RSS |
| Write + reindex single concept | < 100 ms |

## Configuration reference

File: `~/.tahu/config.yaml` (overridden by env vars, then CLI flags)

| Key | Env var | Default | Description |
|---|---|---|---|
| `transport` | `TAHU_TRANSPORT` | `stdio` | `stdio` or `http` |
| `port` | `TAHU_PORT` | `3000` | HTTP mode port |
| `bundle_registry` | `TAHU_REGISTRY` | `~/.tahu/bundles.yaml` | Path to bundle registry file |
| `embedding.model` | `TAHU_EMBED_MODEL` | `minilm-l6-v2` | `minilm-l6-v2` or `bm25` |
| `embedding.batch_size` | `TAHU_EMBED_BATCH` | `32` | Embedding batch size |
| `index.hnsw_ef_construction` | — | `200` | HNSW build-time search depth |
| `index.hnsw_m` | — | `16` | HNSW max connections per node |
| `log_level` | `TAHU_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

---

*For architectural decisions see [`arch-decisions-record.md`](arch-decisions-record.md).*
