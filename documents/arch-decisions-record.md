# Architecture Decision Record

Append-only log of architectural decisions. Add a new entry for every architecturally significant change. Never modify past entries — supersede them with a new ADR.

Format per entry:
```
## ADR-NNN: <title>
**Date:** YYYY-MM-DD
**Status:** Proposed | Accepted | Superseded by ADR-NNN
**Context:** why this decision was needed
**Decision:** what was decided
**Consequences:** trade-offs, constraints imposed
**Alternatives considered:** what was rejected and why
```

---

## ADR-001: Clean Architecture as the structural pattern

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** The daemon needs to support multiple embedding strategies (ONNX, BM25), multiple MCP transports (stdio, HTTP), and pluggable vector stores without business logic leaking into transport or SDK layers.

**Decision:** Adopt Clean Architecture with four layers: `domain` (no external deps), `usecase` (orchestration), `adapter` (interface implementations), `infra` (transport + third-party SDKs). Dependency rule: inner layers never import outer layers.

**Consequences:** All domain interfaces are defined in `internal/domain/`. Third-party SDK imports are confined to `internal/infra/` and `internal/adapter/`. Tests can use in-process fakes for domain interfaces without involving real storage or network.

**Alternatives considered:** Hexagonal (ports and adapters) — structurally equivalent; Clean Architecture naming chosen for clarity with the team. Layerless flat package structure — rejected; no enforcement of dependency direction.

---

## ADR-002: OKF v0.1 as the sole document format

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** The daemon manages knowledge bases for AI agents. A standard format is needed so bundles are portable and human-readable without proprietary tooling.

**Decision:** Support only OKF v0.1 (Apache 2.0, `github.com/GoogleCloudPlatform/knowledge-catalog`). Bundles are directories of UTF-8 markdown files with YAML frontmatter. Only `type` is required. `index.md` and `log.md` are reserved.

**Consequences:** Relationships are plain markdown hyperlinks — no RDF, no typed edges. The daemon must enforce OKF write invariants (reserved filenames, required `type`). Non-OKF formats (HTML, DOCX, PDF) are explicitly out of scope for v0.1.

**Alternatives considered:** Custom JSON schema — rejected; not human-editable without tooling. RDF/OWL — rejected; over-complex for the "LLM wiki" pattern OKF formalizes.

---

## ADR-003: Zero external services for search — dual embedding tier

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** The user requirement is that semantic/vector/RAG search must work with no external service dependencies (no cloud API, no separately running vector database). However, "no external services" and "no CGo" are different constraints.

**Decision:** Two embedding tiers behind a single `domain.Embedder` interface:
1. **ONNX MiniLM-L6-v2** (default) — compiled-in model via `go:embed`, loaded via `github.com/yalue/onnxruntime_go`. Requires `libonnxruntime` native lib at link time (CGo). No network at query time.
2. **Pure-Go BM25** (fallback) — Okapi BM25 keyword index. Zero CGo, zero native deps. Works on Alpine musl.

The ONNX tier satisfies the quality requirement. The BM25 tier satisfies the zero-native-dependency requirement. Operators choose via config.

**Consequences:** Builds targeting pure-Go environments use BM25 and accept lower retrieval quality. The binary is not universally single-binary for all targets — the ONNX tier links `libonnxruntime`. The `domain.Embedder` interface is identical for both tiers; switching is a config change.

**Alternatives considered:** Ollama — rejected; requires an external running process. Cloud embedding API — rejected; introduces network dependency at query time. Pure-Go sentence transformer (no CGo) — no production-quality implementation found at time of decision; revisit when available (see OQ-1).

---

## ADR-004: MCP as the primary agent interface

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** The daemon needs to serve both interactive CLI agents (Claude Code, aider) and automated orchestration pipelines (LangGraph, custom multi-agent systems) from a single binary.

**Decision:** Expose all capabilities exclusively through MCP tools (14 tools, v0.1 surface). Support two MCP transports from the same binary: stdio (default, for CLI agents) and HTTP/SSE (for orchestration). No REST API, no gRPC.

**Consequences:** Any client that speaks MCP can use the daemon. Tool surface is the contract — changes to tool names or schemas are breaking changes. The daemon does not call LLMs; it only provides retrieval. Synthesis is the caller's responsibility.

**Alternatives considered:** gRPC — rejected; requires proto codegen and client stubs. REST — rejected; no standard tool-discovery mechanism. Direct Go library import — supported via `pkg/` for embedding use cases, but not the primary interface.

---

## ADR-005: Disk-backed HNSW as the vector index

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** Vector indexes must survive daemon restarts (G7) and must not require an external vector database service.

**Decision:** Use a pure-Go HNSW implementation persisted to `<bundle-root>/.tahu/vectors.bin`. The index is memory-mapped on read and updated incrementally on writes. Full rebuild is available via `bundle_reindex`.

**Consequences:** Index files live alongside the bundle and are gitignored. Memory-mapped I/O bounds RSS proportional to index size (~200 bytes × 512 dims × concept count). No separate process or service needed.

**Alternatives considered:** SQLite with vector extension — introduces CGo dependency and extension management. In-memory only — fails G7 (no restart survival). External vector DB (Qdrant, Weaviate, Pinecone) — fails the zero-external-services requirement.
