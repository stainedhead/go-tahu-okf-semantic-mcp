# Product Summary

> One-page executive view. Update whenever the value proposition, target users, or core capability set changes.

## What it is

`go-tahu-okf-semantic-mcp` is a knowledge-management daemon for AI agents. It manages one or more [OKF](https://okf.md) (Open Knowledge Format) bundles — hierarchical directories of markdown-based concept documents — and exposes them via an MCP (Model Context Protocol) server.

## Who uses it

| Actor | How |
|---|---|
| CLI agents (Claude Code, aider) | MCP over stdio — the daemon is launched as a subprocess |
| Orchestration agents (LangGraph, custom pipelines) | MCP over HTTP/SSE |
| Human operators | `tahu` CLI sub-commands |

## Core value

Agents get structured tools to **read, write, navigate, and semantically search** OKF knowledge bases with no cloud account, no external vector database, and no network call at query time. The daemon is a single Go binary.

## Key capabilities

- Multi-bundle registry — manage any number of named OKF bundles
- Full OKF read/write — concept CRUD, `index.md` / `log.md` management, link-graph traversal
- Semantic search — vector similarity (ONNX MiniLM, compiled-in) or BM25 (pure-Go fallback)
- RAG retrieval — top-K ranked chunks with source attribution, scoped to global / bundle / sub-path
- Dual MCP transport — stdio for CLI agents, HTTP/SSE for orchestration agents

## Non-goals

Not a web UI. Not a cloud service. Not a document editor. Not an LLM (the daemon retrieves; agents synthesize).

---

*For full product context see [`product-details.md`](product-details.md). For the full PRD see [`../go-tahu-okf-semantic-mcp-PRD.md`](../go-tahu-okf-semantic-mcp-PRD.md).*
