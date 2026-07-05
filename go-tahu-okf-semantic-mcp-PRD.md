# go-tahu-okf-semantic-mcp — Product Requirements Document

**Version:** 0.1-draft  
**Date:** 2026-07-05  
**Authors:** Product / Architecture  
**Status:** Draft — under review

---

## 1. Executive Summary

`go-tahu-okf-semantic-mcp` is a knowledge-management daemon written in Go that manages one or more **OKF (Open Knowledge Format) bundles** — hierarchical directory trees of markdown-based concept documents. The daemon exposes its capabilities through a **Model Context Protocol (MCP) server**, giving AI agents (Claude, GPT, Gemini, custom orchestrators) structured tools to read, write, navigate, and semantically search those knowledge bases.

All search capabilities — including vector embedding and RAG retrieval — operate with **zero external service dependencies**. Embeddings and vector indexes live on disk alongside the OKF bundles. No cloud API, separate vector database, or network call is required at query time.

---

## 2. Background — OKF Primer

OKF v0.1 is an Apache 2.0 open standard published June 12, 2026 by Google Cloud (authors: Sam McVeety & Amir Hormati). Canonical spec: `github.com/GoogleCloudPlatform/knowledge-catalog`.

### 2.1 Bundle structure

```
my-knowledge-base/          ← bundle root
  index.md                  ← RESERVED: directory listing (progressive disclosure)
  log.md                    ← RESERVED: chronological update history
  bigquery-table.md         ← concept document
  runbooks/
    index.md
    deploy-pipeline.md
    rollback.md
  apis/
    index.md
    payments-api.md
```

- A **bundle** is a directory tree. Sub-directories are nested bundles.
- Every file except `index.md` and `log.md` is a **concept document**.
- Concept documents are UTF-8 markdown with YAML frontmatter.

### 2.2 Concept document schema

```yaml
---
type: BigQuery Table          # REQUIRED — free-form string; producers choose descriptive values
title: Orders Fact Table      # optional but recommended
description: |                # optional but recommended
  Daily aggregated orders...
resource: bq://proj/ds/orders # optional — URI of the described resource
tags: [analytics, billing]    # optional
timestamp: 2026-06-15T10:00Z  # optional — last meaningful update
---

# Orders Fact Table

Full markdown prose here. Links to related concepts use standard markdown:
see [Payments API](../apis/payments-api.md).
```

**Conformance rule**: consumers MUST NOT reject a bundle for missing optional fields, unknown `type` values, unknown frontmatter keys, or broken cross-links.

### 2.3 Relationships

Concept-to-concept relationships are expressed as **plain markdown hyperlinks**. The kind of relationship is conveyed by surrounding prose, not by a typed edge. There is no RDF, OWL, or SPARQL in the OKF spec.

### 2.4 `index.md` and `log.md`

| File | Purpose |
|---|---|
| `index.md` | Human- and agent-readable directory listing; acts as the entry point for progressive disclosure |
| `log.md` | Chronological update history for the directory; entries grouped by date, newest first |

---

## 3. Problem Statement

AI agents — both interactive CLI agents and automated orchestration pipelines — need to:
1. **Discover** what knowledge exists across one or more OKF bundles.
2. **Read** specific concept documents or navigate the hierarchy.
3. **Write** new or updated concepts while preserving OKF structural invariants.
4. **Search semantically** — "find all concepts related to billing retry logic" — without requiring agents to know the bundle layout or enumerate files manually.
5. **Scope searches** — to a single bundle, a sub-directory within a bundle, or across all managed bundles simultaneously.

No known tool provides all of these capabilities in a single zero-dependency binary. Known tooling (`kcmd`, `mfdaves/okf-mcp`) appears limited in scope based on available information, though a full competitive survey has not been completed.

---

## 4. Users and Actors

| Actor | Description |
|---|---|
| **CLI agent** | A human-driven AI agent (e.g. Claude Code, aider) interacting with the daemon via MCP over stdio |
| **Orchestration agent** | An automated pipeline (LangGraph, multi-agent system) calling MCP tools over HTTP/SSE |
| **Human operator** | The person who configures, runs, and monitors the daemon; uses the CLI sub-commands directly |
| **Bundle author** | A human or agent creating/editing OKF concept documents |

---

## 5. Goals and Non-Goals

### Goals

- G1: Manage any number of OKF bundles registered with the daemon (add, remove, list).
- G2: Expose the full OKF surface (read, write, navigate, index/log access) via MCP tools.
- G3: Provide semantic similarity search and keyword search over any scope (all bundles, one bundle, any sub-directory) with zero external service dependencies.
- G4: Maintain OKF structural correctness (reserved filenames, frontmatter validation) on all write operations.
- G5: Ship as a single self-contained Go binary — no installation of Python, Node, Docker, or any cloud SDK required.
- G6: Support both **stdio MCP** (for CLI agents) and **HTTP/SSE MCP** (for orchestration agents) from the same binary.
- G7: Persist vector indexes on disk; indexes survive daemon restarts without re-embedding.
- G8: Allow per-bundle and cross-bundle RAG queries that return ranked chunks with source attribution.

### Non-Goals

- NG1: Real-time collaborative editing or conflict resolution (git is the source of truth; the daemon is a reader/writer).
- NG2: Providing a web UI — the MCP interface is the product.
- NG3: Cloud-hosted deployment (the daemon runs wherever the operator runs it).
- NG4: Supporting non-OKF document formats (e.g. raw HTML, DOCX, PDF).
- NG5: Typed semantic edges (OKF uses prose links; we respect the spec).

---

## 6. Core Capabilities

### 6.1 Bundle Registry

The daemon maintains a **bundle registry** — a config file (default: `~/.tahu/bundles.yaml`) that records:
- A user-assigned alias for each bundle.
- The filesystem path to the bundle root.
- Optional metadata (description, tags).

Bundles can be added/removed at runtime without restarting the daemon.

### 6.2 OKF Navigation and Read

Given a bundle alias and an optional relative path, the daemon can:
- Return the concept document at that path.
- List all concepts in a directory (via `index.md` or by scanning).
- Return the `log.md` for any directory level.
- Follow markdown hyperlinks between concepts to return linked documents.
- Return the link graph (outbound links) for a concept — enabling agents to traverse the knowledge graph hop by hop.

### 6.3 OKF Write

The daemon validates and writes concept documents:
- Enforces that `type` is present in frontmatter.
- Prevents writing to `index.md` or `log.md` paths via the concept write tool (separate tools manage those).
- Auto-regenerates `index.md` for a directory after any write.
- Appends a timestamped entry to `log.md` on any write.
- Normalizes YAML frontmatter key order (type, title, description, resource, tags, timestamp).

### 6.4 Semantic / Vector Search

#### Embedding strategy

"No external dependencies" means **no external services** — no cloud API, no separately running vector database, no network call at query time. Native library linkage (CGo) is acceptable provided the binary remains self-contained after `go build`.

Two embedding tiers are supported, selected by config:

| Tier | Mechanism | Dependency | Use case |
|---|---|---|---|
| **Semantic** (default) | ONNX sentence-transformer (e.g. `all-MiniLM-L6-v2`, ~22 MB) via `github.com/yalue/onnxruntime_go`; model file embedded with `go:embed` | CGo + `libonnxruntime` native lib (compiled in or sidecar) | Linux/macOS targets where CGo is available |
| **Keyword** (pure-Go fallback) | Okapi BM25 index; zero native deps, zero CGo | None | Alpine musl, pure-Go targets, or operator preference |

The pure-Go BM25 tier satisfies G5 (single binary, no external install) unconditionally. The ONNX semantic tier is strongly preferred for retrieval quality but requires a native `libonnxruntime` — this is a build-time decision, not a runtime service dependency. OQ-1 and OQ-3 (section 12) track the model selection and fallback sufficiency questions.

Both tiers share the same `domain.Embedder` interface; the operator chooses which tier to activate.

#### Chunking

Each concept document is split into chunks:
- Frontmatter fields are one chunk (type, title, description, tags).
- Markdown prose is split into paragraph-level chunks (≤ 512 tokens each).
- Each chunk carries metadata: bundle alias, relative path, chunk index, frontmatter summary.

#### Vector index

Chunks are indexed in a **pure-Go HNSW** (Hierarchical Navigable Small World) index persisted to disk at `<bundle-root>/.tahu/vectors.bin`. The index is built lazily on first query and incrementally updated on writes.

#### Search scopes

| Scope | Description |
|---|---|
| `global` | Search across all registered bundles |
| `bundle:<alias>` | Search within one bundle |
| `path:<alias>:<relative-path>` | Search within a specific sub-directory |

#### RAG query

A RAG query takes a natural-language question, retrieves the top-K most relevant chunks, and returns them as a ranked list of `{source, chunk_text, score}` objects. The daemon does **not** call an LLM for answer synthesis — that is the calling agent's responsibility. The daemon provides the retrieval layer only.

### 6.5 MCP Tool Surface

All tools follow the MCP specification. Input/output schemas are JSON.

| Tool | Description |
|---|---|
| `bundle_list` | List all registered bundles with alias, path, and concept count |
| `bundle_add` | Register a new bundle by path and alias |
| `bundle_remove` | Unregister a bundle (does not delete files) |
| `bundle_reindex` | Force a full re-embed and re-index of a bundle |
| `concept_read` | Read a concept document (returns frontmatter + body) |
| `concept_write` | Create or update a concept document |
| `concept_list` | List concepts in a directory level |
| `concept_links` | Return outbound markdown links from a concept |
| `index_read` | Read the `index.md` for a directory |
| `log_read` | Read the `log.md` for a directory |
| `search_semantic` | Vector similarity search (returns ranked chunks) |
| `search_keyword` | BM25 keyword search (returns ranked chunks) |
| `search_rag` | Retrieve top-K chunks for a natural-language query across a scope |
| `concept_type_list` | List all distinct `type` values used in a bundle |

### 6.6 CLI Interface

The daemon binary also exposes a `tahu` CLI for operator use:

```
tahu serve               # Start the MCP server (stdio or HTTP/SSE based on flags)
tahu bundle list         # List registered bundles
tahu bundle add <path>   # Register a bundle
tahu bundle reindex      # Force re-embed a bundle
tahu search <query>      # Run a semantic search from the terminal
tahu concept read <ref>  # Read a concept (ref = alias:relative/path.md)
```

---

## 7. MCP Transport

| Mode | Flag | Use case |
|---|---|---|
| stdio | `--transport stdio` (default) | CLI agents (Claude Code, aider) — launched as a subprocess |
| HTTP + SSE | `--transport http --port 3000` | Orchestration agents, multi-agent pipelines |

Both modes expose the same tool surface. The daemon auto-detects transport from flags; no code change needed to switch.

---

## 8. Non-Functional Requirements

| Requirement | Target |
|---|---|
| Zero external services at query time | Hard requirement — all search runs in-process |
| Single binary | No external services required; pure-Go BM25 tier requires only `go build`; ONNX semantic tier requires `libonnxruntime` at link time |
| Startup time | < 500 ms for daemon with up to 10 bundles registered |
| Search latency (semantic, 10k concepts) | p99 < 200 ms |
| Memory (10k concepts, 512-dim embeddings) | < 512 MB RSS |
| Index persistence | Survives daemon restart; incremental update on write |
| OKF spec conformance | All writes validated; no reserved filename violations |
| Concurrency | Thread-safe reads; serialized writes per bundle |

---

## 9. Configuration

The daemon reads config from (in precedence order): CLI flags → env vars → config file.

Config file default: `~/.tahu/config.yaml`

```yaml
transport: stdio          # stdio | http
port: 3000                # HTTP mode only
bundle_registry: ~/.tahu/bundles.yaml
embedding:
  model: minilm-l6-v2     # compiled-in model alias; "bm25" for fallback
  batch_size: 32
index:
  hnsw_ef_construction: 200
  hnsw_m: 16
log_level: info
```

---

## 10. Data Model

```
BundleRegistry
  bundles: []BundleEntry
    alias: string            # user-assigned unique identifier
    root_path: string        # absolute filesystem path
    description: string
    tags: []string
    created_at: time.Time
    last_indexed_at: time.Time

OKFConcept
  path: string               # relative path within bundle
  frontmatter:
    type: string             # required
    title: string
    description: string
    resource: string
    tags: []string
    timestamp: time.Time
  body: string               # markdown prose after frontmatter
  outbound_links: []string   # resolved relative paths of markdown links

EmbeddingChunk
  id: string                 # bundle_alias:path:chunk_index
  bundle_alias: string
  concept_path: string
  chunk_index: int
  text: string
  embedding: []float32
  frontmatter_summary: string
```

---

## 11. Out of Scope (v0.1)

- LLM-based answer synthesis (agents do their own synthesis with retrieved chunks).
- OKF bundle validation / linting beyond write-time checks.
- Git integration (commit, push, diff).
- Authentication / authorization on the MCP server.
- Windows support (Linux and macOS only for v0.1).
- Bundle replication or sync between daemon instances.

---

## 12. Open Questions

| # | Question | Owner | Status |
|---|---|---|---|
| OQ-1 | Which ONNX sentence-transformer model gives the best accuracy/size tradeoff for OKF concept prose? | Eng | Open |
| OQ-2 | Should `concept_write` auto-generate `index.md` always, or only when one already exists? | Product | Open |
| OQ-3 | Is BM25 fallback sufficient for environments without CGo (e.g. Alpine musl)? | Eng | Open |
| OQ-4 | Should the daemon support watching the filesystem for external edits and auto-reindexing? | Product | Open |
| OQ-5 | What is the right chunk overlap strategy for OKF concept prose vs. frontmatter? | Eng | Open |

---

## 13. References

- OKF SPEC: `github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md`
- OKF site: `okf.md`
- Google Cloud blog announcement: June 12, 2026
- Existing MCP reference: `github.com/mfdaves/okf-mcp`
- ONNX Runtime Go: `github.com/yalue/onnxruntime_go`
- MCP specification: `modelcontextprotocol.io`
