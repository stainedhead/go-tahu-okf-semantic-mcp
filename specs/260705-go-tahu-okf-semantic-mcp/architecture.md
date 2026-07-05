# Architecture: go-tahu-okf-semantic-mcp

**Feature:** go-tahu-okf-semantic-mcp  
**Date:** 2026-07-05  
**Status:** Draft

---

## Architecture Overview

Clean Architecture — dependency rule: inner rings never import outer rings.

```
┌─────────────────────────────────────────────────┐
│  cmd/tahu (entry point)                         │
│  ┌───────────────────────────────────────────┐  │
│  │  infra/ (transport, config, registry)     │  │
│  │  ┌─────────────────────────────────────┐  │  │
│  │  │  adapter/ (mcp, okf, embedder,      │  │  │
│  │  │           vectorstore)              │  │  │
│  │  │  ┌───────────────────────────────┐  │  │  │
│  │  │  │  usecase/                     │  │  │  │
│  │  │  │  ┌─────────────────────────┐  │  │  │  │
│  │  │  │  │  domain/                │  │  │  │  │
│  │  │  │  │  (entities+interfaces)  │  │  │  │  │
│  │  │  │  └─────────────────────────┘  │  │  │  │
│  │  │  └───────────────────────────────┘  │  │  │
│  │  └─────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

---

## Component Architecture

### `internal/domain/`
- `bundle.go` — BundleEntry, BundleRegistry
- `concept.go` — OKFConcept, OKFFrontmatter, ConceptRef, ConceptLink
- `chunk.go` — EmbeddingChunk, ScoredChunk, Scope, ScopeKind
- `errors.go` — sentinel domain errors
- `interfaces.go` — Embedder, VectorStore, NodeRepository, BundleRepository
- **Zero external imports.** All tests use in-memory fakes.

### `internal/usecase/`
- `bundle.go` — AddBundle, RemoveBundle, ListBundles, ReindexBundle
- `concept.go` — ReadConcept, WriteConcept, ListConcepts, GetLinks, ReadIndex, ReadLog, ListTypes
- `search.go` — SemanticSearch, KeywordSearch, RAGSearch
- **Depends only on domain interfaces.** Fakes injected in tests.

### `internal/adapter/okf/`
- `parser.go` — parse frontmatter (yaml.v3) + body from `.md` file bytes
- `linker.go` — extract markdown links using goldmark AST walker
- `indexer.go` — generate `index.md` listing; append `log.md` entry
- `validator.go` — reserved-filename check, type presence, path confinement

### `internal/adapter/embedder/`
- `bm25.go` — Okapi BM25 index over chunk texts; implements `domain.Embedder`
- `onnx.go` — ONNX Runtime Go MiniLM loader; implements `domain.Embedder`
- `chunker.go` — split OKFConcept into EmbeddingChunks (frontmatter chunk + paragraph chunks)

### `internal/adapter/vectorstore/`
- `hnsw.go` — `coder/hnsw` wrapper; disk persistence at `<bundle-root>/.tahu/vectors.bin`; implements `domain.VectorStore`

### `internal/adapter/mcp/`
- `schema.go` — JSON Schema definitions for all 14 tools
- `validation.go` — input size caps, path parsing, scope parsing
- `handlers.go` — one function per MCP tool; thin delegation to usecase
- `tools.go` — MCP tool registration (names, descriptions, schemas, handlers)

### `internal/infra/transport/`
- `stdio.go` — JSON-RPC 2.0 over stdin/stdout
- `http.go` — HTTP POST + SSE + `GET /healthz`

### `internal/infra/config/`
- `config.go` — Config struct; load order: file → env → flags

### `internal/infra/registry/`
- `yaml.go` — BundleRepository backed by `~/.tahu/bundles.yaml`

### `cmd/tahu/`
- `main.go` — cobra CLI; DI wiring; start server

---

## Layer Responsibilities

| Layer | Knows about | Never imports |
|---|---|---|
| domain | itself | everything else |
| usecase | domain | adapter, infra, cmd |
| adapter | domain, usecase | infra, cmd |
| infra | domain, usecase, adapter | cmd |
| cmd | everything | (is the outer shell) |

---

## Data Flow

### Write concept
```
Agent → MCP tool: concept_write(ref, frontmatter, body)
  → adapter/mcp/validation: path confinement + size cap
  → usecase/concept.WriteConcept(ctx, ref, concept)
    → adapter/okf/validator: required type, reserved filename
    → domain.NodeRepository.Put(ctx, ref, concept)
      → adapter/okf/parser: serialize frontmatter + body → disk
    → adapter/okf/indexer: regenerate index.md
    → adapter/okf/indexer: append log.md entry
    → adapter/embedder: Embed(ctx, [chunk texts])
    → domain.VectorStore.Upsert(ctx, chunks)
      → adapter/vectorstore/hnsw: update HNSW index + persist
← MCP response: success
```

### RAG search
```
Agent → MCP tool: search_rag(query, scope, top_k, min_score)
  → adapter/mcp/validation: scope parse, parameter bounds
  → usecase/search.RAGSearch(ctx, query, scope, top_k, min_score)
    → domain.Embedder.Embed(ctx, [query])           // in-process, no network
    → domain.VectorStore.Search(ctx, vec, scope, top_k)  // disk HNSW
    → filter by min_score
← MCP response: []ScoredChunk
```

---

## Sequence Diagrams

_[To be added during architecture phase — key flows: bundle_add + full index, concept_write + incremental index update, search_rag cold start]_

---

## Integration Points

| Integration | Protocol | Direction |
|---|---|---|
| CLI agent (Claude Code) | MCP over stdio (JSON-RPC 2.0) | Bidirectional |
| Orchestration agent | MCP over HTTP/SSE | Bidirectional |
| OKF bundle on disk | Filesystem (read/write `.md` files) | Bidirectional |
| Bundle registry | YAML file at `~/.tahu/bundles.yaml` | Bidirectional |
| HNSW index | Binary file at `<bundle>/.tahu/vectors.bin` | Bidirectional |

---

## Architectural Decisions

See `documents/arch-decisions-record.md` for ADR-001 through ADR-005.

Key decisions affecting this spec:
- **ADR-001** Clean Architecture — enforced via import path rules
- **ADR-002** OKF-only format — no format adapters needed
- **ADR-003** Dual embedding tier — both tiers behind `domain.Embedder`
- **ADR-004** MCP-only interface — no REST/gRPC
- **ADR-005** Disk-backed HNSW — no external vector DB
