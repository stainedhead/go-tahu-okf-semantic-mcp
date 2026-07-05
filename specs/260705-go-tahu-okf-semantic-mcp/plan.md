# Plan: go-tahu-okf-semantic-mcp

**Feature:** go-tahu-okf-semantic-mcp  
**Date:** 2026-07-05  
**Status:** Planning

---

## Development Approach

TDD throughout: Red → Green → Refactor. Build inward-out following Clean Architecture:
1. Domain (no deps) — types and interfaces
2. Use cases (depend on domain interfaces via fakes)
3. Adapters (implement domain interfaces; integration tests)
4. Infra / transport (wire everything; end-to-end tests)
5. `cmd/tahu` entry point and CLI

Resolve RQ-3 (MCP SDK) and RQ-5 (HNSW persistence) before starting adapter work — both are blockers.

---

## Phase Breakdown

### Phase 1 — Domain layer
- Domain types: BundleEntry, OKFConcept, OKFFrontmatter, ConceptRef, ConceptLink, EmbeddingChunk, ScoredChunk, Scope
- Domain interfaces: Embedder, VectorStore, NodeRepository, BundleRepository
- Sentinel errors
- In-memory fakes for all interfaces (used by use case tests)
- **Exit gate:** `go test -race ./internal/domain/...` passes; zero external imports in domain

### Phase 2 — Use cases
- Bundle: AddBundle, RemoveBundle, ListBundles, ReindexBundle
- Concept: ReadConcept, WriteConcept, ListConcepts, GetLinks, ReadIndex, ReadLog, ListTypes
- Search: SemanticSearch, KeywordSearch, RAGSearch
- All tested with fakes; spec acceptance criteria G1–G8 encoded as test cases
- **Exit gate:** `go test -race ./internal/usecase/...` passes; no imports below adapter layer

### Phase 3 — OKF adapter
- `parser.go` — yaml.v3 frontmatter + markdown body; round-trip parse/serialize
- `linker.go` — goldmark AST link extraction; broken-link detection
- `indexer.go` — index.md generation; log.md append
- `validator.go` — reserved filenames, type presence, path confinement
- Integration tests using real `.md` fixtures in `testdata/`
- **Exit gate:** `go test -tags integration -race ./internal/adapter/okf/...` passes

### Phase 4 — Embedder adapters
- `chunker.go` — frontmatter chunk + paragraph chunks, configurable overlap
- `bm25.go` — Okapi BM25 (zero CGo); passes RQ-2 benchmark
- `onnx.go` — ONNX MiniLM (CGo + libonnxruntime); passes RQ-1 benchmark
- **Exit gate:** BM25 embeds correctly with `CGO_ENABLED=0`; both tiers implement `domain.Embedder`

### Phase 5 — Vector store adapter
- `hnsw.go` — coder/hnsw wrapper; disk persistence; lazy build; incremental update
- Scope filtering (global / bundle / path prefix)
- Integration tests with real HNSW index file
- **Exit gate:** `go test -tags integration -race ./internal/adapter/vectorstore/...` passes; index survives process restart

### Phase 6 — MCP adapter
- `schema.go` — JSON Schema for all 14 tools
- `validation.go` — path confinement, size caps, scope parsing
- `handlers.go` + `tools.go` — thin handler registration
- Unit tests: each handler delegates correctly; validation rejects bad inputs
- **Exit gate:** all 14 tools registered; validation tests pass

### Phase 7 — Infra: transport + config + registry
- stdio and HTTP/SSE MCP transports; `GET /healthz`
- Config loader (file → env → flags)
- YAML bundle registry persistence
- **Exit gate:** both transports serve all 14 tools; config precedence tests pass

### Phase 8 — `cmd/tahu` entry point and CLI
- cobra CLI: `serve`, `bundle list/add/reindex`, `search`, `concept read`
- Manual DI wiring in `main.go`
- End-to-end smoke test: `tahu bundle add + tahu search`
- **Exit gate:** `go build ./cmd/tahu` succeeds; smoke test passes on Linux + macOS

---

## Critical Path

RQ-3 (MCP SDK) → Phase 6 → Phase 7 → Phase 8  
RQ-5 (HNSW persistence) → Phase 5 → Phase 8

All other phases (1, 2, 3, 4) can proceed in parallel once domain is stable.

---

## Testing Strategy

| Test type | Location | Runner |
|---|---|---|
| Unit (no I/O) | `*_test.go` alongside source, `package foo` or `package foo_test` | `go test -race ./...` |
| Integration (I/O, real files) | `*_integration_test.go`, `//go:build integration` | `go test -tags integration -race ./...` |
| End-to-end | `cmd/tahu/*_e2e_test.go`, `//go:build e2e` | `go test -tags e2e ./cmd/...` |

Test naming: `TestFunctionName_SpecRequirement` (e.g. `TestWriteConcept_RejectsReservedPath_FR011`).

---

## Rollout Strategy

Greenfield — no migration, no feature flag. Binary is the deliverable.  
Post-PR: operator installs binary, runs `tahu bundle add`, verifies `tahu search`.

---

## Success Metrics

- All acceptance criteria in `spec.md` pass as automated tests
- `go test -race ./...` clean
- `golangci-lint run ./...` clean
- Domain + Use Case coverage ≥ 90%
- `go build ./cmd/tahu` succeeds on Linux amd64 + macOS arm64
