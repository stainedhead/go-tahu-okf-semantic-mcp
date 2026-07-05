# Product Summary

> One-page executive view. Update whenever the value proposition, target users, or core capability set changes.

## What it is

`tahu` is a knowledge-management daemon for AI agents. It manages one or more **OKF** (Open Knowledge Format v0.1) bundles — hierarchical directories of UTF-8 markdown files with YAML frontmatter — and exposes them through an **MCP** (Model Context Protocol) server.

The daemon is a single Go binary with zero external service dependencies. All search and indexing runs in-process.

## Who uses it

| Actor | How |
|---|---|
| CLI agents (Claude Code, aider) | MCP over stdio — the daemon is launched as a subprocess |
| Orchestration agents (LangGraph, custom pipelines) | MCP over HTTP/SSE |
| Human operators | `tahu` CLI subcommands |

## Core value

Agents get structured tools to **read, write, navigate, and semantically search** OKF knowledge bases with no cloud account, no external vector database, and no network call at query time.

## Key capabilities

- **Multi-bundle registry** — register any number of named OKF bundles by filesystem path
- **Full OKF read/write** — concept CRUD, `index.md`/`log.md` management, link-graph traversal
- **In-process semantic search** — BM25 keyword embeddings indexed in a disk-backed HNSW vector graph; zero CGo, zero native dependencies
- **RAG retrieval** — top-K ranked chunks with source attribution, scoped to global / bundle / sub-path
- **Dual MCP transport** — stdio for CLI agents, HTTP/SSE for orchestration agents

## Embedding tier (v0.1)

The sole embedding backend is a pure-Go BM25 implementation. A dense ONNX MiniLM-L6-v2 tier is planned but deferred to a future release.

## Non-goals

Not a web UI. Not a cloud service. Not a document editor. Not an LLM (tahu retrieves; agents synthesize).

---

*For full product context see [`product-details.md`](product-details.md).*  
*For technical implementation details see [`technical-details.md`](technical-details.md).*
