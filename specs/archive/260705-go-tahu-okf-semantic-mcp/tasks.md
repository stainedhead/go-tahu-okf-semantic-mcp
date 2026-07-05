# Tasks: go-tahu-okf-semantic-mcp

**Feature:** go-tahu-okf-semantic-mcp  
**Date:** 2026-07-05  
**Status:** Planning

---

## Progress Summary

**0 / 0 tasks complete** (tasks to be broken down per phase as each phase begins)

---

## Phase 1 — Domain Layer

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P1.1 | Define domain types: BundleEntry, OKFConcept, OKFFrontmatter, ConceptRef, ConceptLink | — | 1 | ⬜ | Types compile; all fields match data-dictionary.md |
| P1.2 | Define domain types: EmbeddingChunk, ScoredChunk, Scope, ScopeKind | P1.1 | 0.5 | ⬜ | Types compile; Scope.String() parses all three kinds |
| P1.3 | Define domain interfaces: Embedder, VectorStore | P1.2 | 0.5 | ⬜ | Interfaces compile; method signatures match data-dictionary.md |
| P1.4 | Define domain interfaces: NodeRepository, BundleRepository | P1.1 | 0.5 | ⬜ | Interfaces compile |
| P1.5 | Define sentinel errors | P1.1 | 0.5 | ⬜ | All errors in data-dictionary.md defined; `errors.Is` works |
| P1.6 | Implement in-memory fakes for all four interfaces | P1.3, P1.4 | 2 | ⬜ | Fakes pass their own unit tests; satisfy interfaces at compile time |
| P1.7 | Gate: `go test -race ./internal/domain/...` passes; zero external imports | P1.6 | — | ⬜ | CI green |

## Phase 2 — Use Cases

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P2.1 | Implement bundle use cases (AddBundle, RemoveBundle, ListBundles, ReindexBundle) | P1.6 | 2 | ⬜ | AC-G1 test cases pass with fakes |
| P2.2 | Implement concept read use cases (ReadConcept, ListConcepts, GetLinks, ReadIndex, ReadLog, ListTypes) | P1.6 | 2 | ⬜ | AC-G2 test cases pass with fakes |
| P2.3 | Implement WriteConcept use case | P1.6 | 1.5 | ⬜ | AC-G4 test cases pass; reserved-filename + missing-type rejection |
| P2.4 | Implement SemanticSearch use case | P1.6 | 1 | ⬜ | AC-G3 scope boundary tests pass with fake VectorStore |
| P2.5 | Implement KeywordSearch use case | P1.6 | 0.5 | ⬜ | Same scope tests as P2.4 |
| P2.6 | Implement RAGSearch use case | P2.4 | 1 | ⬜ | AC-G8 top_k / min_score / empty-result tests pass |
| P2.7 | Gate: `go test -race ./internal/usecase/...` passes | P2.6 | — | ⬜ | CI green; no adapter imports |

## Phase 3 — OKF Adapter

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P3.1 | Implement frontmatter parser (yaml.v3 + markdown body split) | P1.1 | 2 | ⬜ | Round-trip parse/serialize preserves all fields; unknown keys preserved |
| P3.2 | Implement goldmark link extractor | P1.1 | 1.5 | ⬜ | All outbound links extracted; broken links flagged |
| P3.3 | Implement index.md generator | P1.1 | 1 | ⬜ | Generated index lists all non-reserved .md files in directory |
| P3.4 | Implement log.md appender | P1.1 | 0.5 | ⬜ | Entry appended with timestamp; newest-first order maintained |
| P3.5 | Implement path confinement validator | P1.5 | 1 | ⬜ | EvalSymlinks + Clean; escaping paths return ErrPathEscape |
| P3.6 | Integration tests with testdata/ fixtures | P3.5 | 2 | ⬜ | `go test -tags integration` passes |

## Phase 4 — Embedder Adapters

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P4.1 | Implement concept chunker | P3.1 | 1.5 | ⬜ | Frontmatter chunk + paragraph chunks; chunk metadata correct |
| P4.2 | Implement BM25 embedder | P4.1 | 2 | ⬜ | Builds with CGO_ENABLED=0; implements domain.Embedder |
| P4.3 | Implement ONNX MiniLM embedder | P4.1 | 3 | ⬜ | Model loads from go:embed; implements domain.Embedder; latency < 50ms/batch-32 |
| P4.4 | Resolve RQ-1: select ONNX model | P4.3 | 1 | ⬜ | Model selected; RQ-1 closed |

## Phase 5 — Vector Store Adapter

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P5.1 | Resolve RQ-5: spike coder/hnsw persistence | P1.3 | 1 | ⬜ | RQ-5 answered; persistence approach documented in implementation-notes.md |
| P5.2 | Implement HNSW vector store | P5.1, P4.1 | 3 | ⬜ | Upsert + Search + Persist + Load; implements domain.VectorStore |
| P5.3 | Implement scope filtering in Search | P5.2 | 1 | ⬜ | Bundle + path scope filtering returns only matching chunks |
| P5.4 | Integration test: index survives process restart | P5.3 | 1 | ⬜ | `go test -tags integration` passes; same results pre/post restart |

## Phase 6 — MCP Adapter

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P6.1 | Resolve RQ-3: select MCP Go SDK | P1.1 | 1 | ⬜ | SDK selected; RQ-3 closed; go.mod updated |
| P6.2 | Define JSON Schema for all 14 tools | P2.6 | 2 | ⬜ | Schemas compile; cover all input/output fields |
| P6.3 | Implement input validation (size caps, scope parse) | P6.2 | 1.5 | ⬜ | Oversized inputs rejected; invalid scope strings rejected |
| P6.4 | Implement 14 MCP tool handlers | P2.6, P6.3 | 3 | ⬜ | Each handler delegates to correct use case; unit tests pass |
| P6.5 | Register all tools with MCP SDK | P6.4 | 1 | ⬜ | All 14 tools visible in MCP tool list |

## Phase 7 — Infra

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P7.1 | Implement config loader (file → env → flags) | P1.1 | 1.5 | ⬜ | Precedence tests pass; unknown keys ignored |
| P7.2 | Implement YAML bundle registry | P1.4 | 1.5 | ⬜ | Implements BundleRepository; persists to ~/.tahu/bundles.yaml |
| P7.3 | Implement stdio MCP transport | P6.5 | 2 | ⬜ | JSON-RPC 2.0 over stdin/stdout; AC-G6 partial |
| P7.4 | Implement HTTP/SSE MCP transport + /healthz | P6.5 | 2 | ⬜ | Binds 127.0.0.1 by default; /healthz returns 200; AC-G6 complete |

## Phase 8 — cmd/tahu

| ID | Task | Deps | Est (h) | Status | Acceptance |
|---|---|---|---|---|---|
| P8.1 | DI wiring in main.go | P7.4 | 2 | ⬜ | Binary starts; all layers connected |
| P8.2 | cobra CLI: serve, bundle, search, concept | P8.1 | 2 | ⬜ | All CLI subcommands documented in --help; smoke tests pass |
| P8.3 | End-to-end test: add bundle + search | P8.2 | 1 | ⬜ | `go test -tags e2e ./cmd/...` passes on Linux amd64 + macOS arm64 |
| P8.4 | AC-G5: `go build ./cmd/tahu` clean | P8.2 | — | ⬜ | Build succeeds; binary < 50 MB (BM25 tier) |
