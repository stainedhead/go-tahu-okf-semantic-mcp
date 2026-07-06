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

## ADR-003: Zero external services for search — dual embedding tier design

**Date:** 2026-07-05  
**Status:** Accepted — amended by ADR-006 (default tier changed for v0.1)

**Context:** The user requirement is that semantic/vector/RAG search must work with no external service dependencies (no cloud API, no separately running vector database). However, "no external services" and "no CGo" are different constraints.

**Decision:** Design two embedding tiers behind a single `domain.Embedder` interface:
1. **ONNX MiniLM-L6-v2** (planned default) — compiled-in model via `go:embed`, loaded via `github.com/yalue/onnxruntime_go`. Requires `libonnxruntime` native lib at link time (CGo). No network at query time.
2. **Pure-Go BM25** (fallback) — Okapi BM25 keyword index. Zero CGo, zero native deps. Works on Alpine musl.

The ONNX tier satisfies the quality requirement. The BM25 tier satisfies the zero-native-dependency requirement. Operators choose via config.

**Consequences:** Builds targeting pure-Go environments use BM25 and accept lower retrieval quality. The binary is not universally single-binary for all targets — the ONNX tier links `libonnxruntime`. The `domain.Embedder` interface is identical for both tiers; switching is a config change. See ADR-006 for v0.1 shipping decision.

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

**Context:** Vector indexes must survive daemon restarts and must not require an external vector database service.

**Decision:** Use a pure-Go HNSW implementation (`github.com/coder/hnsw`) persisted to a single shared file at `~/.tahu/hnsw.index` (alongside the bundle registry). The index is loaded into memory on startup and updated incrementally on writes. Full rebuild is available via `bundle_reindex`.

**Consequences:** Index survives daemon restarts. A single shared index file simplifies the persistence model — no per-bundle index files to manage. The index is flushed to disk on clean shutdown (SIGINT/SIGTERM). Memory usage is proportional to total concept count across all bundles.

**Alternatives considered:** Per-bundle index files (`<bundle-root>/.tahu/vectors.bin`) — rejected in v0.1 in favour of a single shared file for operational simplicity. SQLite with vector extension — introduces CGo dependency and extension management. In-memory only — fails restart-survival requirement. External vector DB (Qdrant, Weaviate, Pinecone) — fails the zero-external-services requirement.

---

## ADR-006: BM25 as sole embedding tier in v0.1 (ONNX deferred)

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** ADR-003 designed a dual embedding tier and described ONNX MiniLM-L6-v2 as the planned default. During v0.1 implementation, integrating `libonnxruntime` (CGo) was identified as scope risk that would delay the initial release without delivering core functionality.

**Decision:** Ship v0.1 with BM25 as the **only** embedding implementation. The `embedding_model: minilm-l6-v2` config value is accepted and parsed but has no effect — the daemon always uses BM25 in v0.1. The `domain.Embedder` interface and configuration surface are preserved exactly as ADR-003 specified, so adding the ONNX tier in a future release requires no breaking changes.

**Consequences:** v0.1 retrieval quality is keyword-based (BM25), not dense-vector semantic similarity. For most knowledge-base sizes and agent use cases this is adequate. The BM25 embedder produces 4096-dimensional sparse vectors. The ONNX implementation is tracked as a follow-on task.

**Alternatives considered:** Shipping ONNX in v0.1 — rejected; CGo + libonnxruntime dependency management exceeded v0.1 scope. Pure-Go dense embedder (e.g., gorgonia) — no production-quality MiniLM port available at time of decision.

---

## ADR-007: pkg/okfcodec deferred — no external consumer in v0.1

**Date:** 2026-07-05  
**Status:** Accepted

**Context:** AGENTS.md and the original PRD described a `pkg/okfcodec` exportable library so external Go programs could parse and generate OKF documents without running the daemon.

**Decision:** Defer `pkg/okfcodec` implementation entirely in v0.1. The OKF parsing logic lives in `internal/adapter/okf/` and is not exported. No external consumer (agent, pipeline, CI tool) requires the library interface in v0.1. The `pkg/` directory is reserved for this package.

**Consequences:** External Go programs cannot import OKF codec logic without vendoring the internal adapter, which is not a supported pattern. This is an acceptable constraint for v0.1 given zero known consumers. Implementing `pkg/okfcodec` in a future release will require defining a stable public API surface and is therefore a non-trivial design exercise best deferred until consumer requirements are known.

**Alternatives considered:** Shipping a minimal `pkg/okfcodec` stub — rejected; an empty stub creates a false impression of a stable API. Moving all OKF logic to `pkg/` immediately — rejected; premature API stabilisation before consumers exist.

---

## ADR-008: BundlePathResolver as single path-resolution gateway
**Date:** 2026-07-06
**Status:** Accepted

**Context:** Path-traversal and symlink-escape guards were scattered across six methods in `FileNodeRepository`, each duplicating prefix-check logic with slight variations. The `ValidateConceptPath` function in `validator.go` added a seventh variant. No single authoritative boundary existed.

**Decision:** Introduce `internal/adapter/okf/BundlePathResolver` as the single gateway for all filesystem path resolution. All `FileNodeRepository` methods (`Get`, `Put`, `List`, `ReadReserved`, `WriteReserved`) route through it. `ValidateConceptPath` is kept as a defense-in-depth layer for the public MCP handler boundary but no longer the primary enforcement mechanism.

**Consequences:** Path security invariants are testable in isolation (`pathresolver_test.go`). Adding a new file operation requires routing through the resolver, making the security model visible in code review. A regression in the resolver affects all operations uniformly rather than requiring per-method fixes.

**Alternatives considered:** Adding a shared `containedPath` helper called by each method — rejected; callers still need to remember to call it. Using a middleware/decorator around the repository — rejected; the repository interface is the domain boundary; wrapping it in infra creates an import-direction violation.

---

## ADR-009: syscall.Flock advisory lock for the YAML registry
**Date:** 2026-07-06
**Status:** Accepted

**Context:** Concurrent `tahu` processes (e.g., a CLI command running alongside the daemon) can race on the `registry.yaml` file during `Put`/`Delete`. The in-process `sync.Mutex` does not protect against cross-process writes.

**Decision:** Use `syscall.Flock` (Unix-only) as an advisory exclusive file lock around registry mutations. A no-op stub (`flock_windows.go`) is provided for Windows builds. No external dependency is required.

**Consequences:** Unix multi-process safety is achieved with zero new dependencies. Windows is not protected (NG-8 in the spec); a future ADR will address this if Windows support becomes a requirement.

**Alternatives considered:** `flock` library (`github.com/gofrs/flock`) — rejected; avoids adding a dependency for functionality available in stdlib. `os.O_EXCL` lock file — rejected; requires cleanup logic on crash, more complex than advisory locking.
