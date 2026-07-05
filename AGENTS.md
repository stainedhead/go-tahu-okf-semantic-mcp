# AGENTS.md

Centralized rules for all AI coding agents (Claude Code, GitHub Copilot, Cursor, etc.) working in this repository.

---

## Project

`go-tahu-okf-semantic-mcp` is an OKF-based knowledge management daemon. It exposes an MCP (Model Context Protocol) server that gives AI agents tools for reading, writing, and semantically searching OKF knowledge bases using vector embeddings.

**OKF** (Open Knowledge Format) — a graph-structured knowledge representation built on typed nodes, edges, and facets.  
**MCP** — the protocol layer over which agents call tools exposed by this daemon.  
**Semantic search** — embedding-based retrieval over OKF entities using a pluggable vector store.

---

## Architecture — Clean Architecture

Dependency rule: inner layers never import outer layers.

```
cmd/                        ← entry points (wire outer layers, start server)
internal/
  domain/                   ← entities, value objects, repository interfaces, domain errors
  usecase/                  ← application logic; depend only on domain
  adapter/                  ← implementations of domain interfaces (repo impls, MCP handlers, embedder)
  infra/                    ← frameworks, DB drivers, vector store clients, HTTP/MCP transport
pkg/                        ← exportable utilities (OKF codec, embedding helpers)
spec/                       ← feature specs (Markdown, SDD source of truth)
```

- `domain` has zero external dependencies — only stdlib.
- `usecase` orchestrates domain objects; injected via interfaces.
- `adapter` maps between domain objects and external representations (JSON, proto, OKF wire format).
- `infra` is the only place where third-party SDK imports live.

---

## Development Process

### Spec Driven Development (SDD) first

Before writing any code for a feature:
1. Write or update the spec in `spec/<feature>.md` capturing: purpose, inputs/outputs, invariants, edge cases.
2. Get spec reviewed/approved.
3. Derive test cases directly from the spec.

### TDD cycle (RED → GREEN → Refactor)

1. **RED** — write a failing test that encodes one spec requirement.
2. **GREEN** — write the minimum production code to make it pass.
3. **Refactor** — improve structure without breaking tests.

Never write production code without a failing test first.

---

## Go Conventions

- **Idiomatic Go**: prefer composition over inheritance, explicit error returns, context propagation.
- Package names are short, lowercase, no underscores.
- Interfaces are defined in the package that *uses* them (domain/usecase), not in the package that implements them.
- Errors wrap with `%w` and carry context: `fmt.Errorf("store.Get %s: %w", id, err)`.
- Use `context.Context` as the first parameter for all I/O and long-running operations.
- Avoid global state; wire dependencies in `cmd/`.

---

## Build & Test Commands

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run tests with race detector (required before committing)
go test -race ./...

# Run a single test
go test -run TestName ./internal/usecase/...

# Run tests with coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Lint (requires golangci-lint)
golangci-lint run ./...

# Format
gofmt -w .
goimports -w .

# Vet
go vet ./...
```

---

## MCP Server

- MCP tools are registered in `adapter/mcp/` as thin handler functions; business logic lives in `usecase/`.
- Each MCP tool has a corresponding spec in `spec/tools/`.
- Tool input/output schemas are defined as Go structs with JSON tags and validated at the adapter boundary.
- The MCP transport (stdio, SSE, HTTP) is configured in `infra/mcp/`.

---

## OKF Knowledge Model

Core domain types (defined in `internal/domain/`):

| Type | Description |
|---|---|
| `Node` | A typed knowledge entity with a UUID, kind, and facet map |
| `Edge` | A directed, typed relationship between two nodes |
| `Facet` | A typed attribute value on a node or edge |
| `Graph` | A bounded collection of nodes and edges |

Repository interfaces (`domain/repository.go`):
- `NodeRepository` — CRUD + semantic search over nodes
- `GraphRepository` — graph-scoped queries

---

## Vector / Semantic Search

- Embeddings are generated via the `domain.Embedder` interface (implemented in `adapter/embedder/`).
- The vector store is behind the `domain.VectorStore` interface (implemented in `adapter/vectorstore/`).
- Semantic search is a use case in `usecase/search.go` that composes `Embedder` + `VectorStore`.
- Chunking strategy for large OKF graphs is defined per `Node.Kind` in `usecase/chunker.go`.

---

## Testing Conventions

- Unit tests live alongside the code they test (`foo_test.go` in the same package, or `foo_test` package for black-box tests).
- Integration tests that need external dependencies (DB, vector store) live in `internal/adapter/*_integration_test.go` and are gated with `//go:build integration`.
- Use table-driven tests. Name subtests after spec requirements, not implementation details.
- Fakes and stubs live in `internal/domain/testdata/` or `internal/usecase/testdata/` — never use mocks that couple to call order unless the spec explicitly requires it.
- Test the public API of each layer; avoid testing unexported functions.

---

## Language Chain Toolkit

This project integrates with LangChain-compatible tooling for:
- Prompt templating for embedding generation
- LLM-backed OKF entity extraction pipelines
- Retrieval-Augmented Generation (RAG) over OKF graphs

LLM client wrappers live in `adapter/llm/`; the domain defines only `Embedder` and `Extractor` interfaces.

---

## Dependency Injection

All wiring is done in `cmd/<daemon>/main.go` using manual DI (no DI framework). The order:
1. Load config from env/flags.
2. Construct infra clients (DB, vector store, LLM client).
3. Construct adapters (repositories, embedder, extractor).
4. Construct use cases, injecting adapters.
5. Register MCP tool handlers.
6. Start server.

---

## Spec Directory

`spec/` is the source of truth for feature intent:
- `spec/tools/<tool-name>.md` — each MCP tool's contract
- `spec/domain/<entity>.md` — OKF entity invariants
- `spec/usecases/<usecase>.md` — application-level flows

Test case names in `_test.go` files should reference the spec section they validate (e.g., `TestSearchNodes_ReturnsTopK_SpecSearch3`).

---

## Living Documentation

Two locations that must stay current with the codebase. **Update both whenever a change is architecturally significant** (new layer, new external dependency, changed data model, new MCP tool surface, changed deployment topology, ADR).

### `documents/`

| File | Purpose |
|---|---|
| `product-summary.md` | One-page executive view: what the daemon does, who uses it, core value proposition |
| `product-details.md` | Full product context: user stories, capability map, integration points, non-goals |
| `technical-details.md` | Technical deep-dive: data flows, protocol details, performance characteristics, configuration reference |
| `arch-decisions-record.md` | Append-only log of Architecture Decision Records (ADRs). Each ADR captures: context, decision, consequences, alternatives rejected |

ADR format in `arch-decisions-record.md`:

```
## ADR-NNN: <title>
**Date:** YYYY-MM-DD
**Status:** Proposed | Accepted | Superseded by ADR-NNN
**Context:** <why this decision was needed>
**Decision:** <what was decided>
**Consequences:** <trade-offs, constraints imposed>
**Alternatives considered:** <what was rejected and why>
```

### `README.md`

Root-level README targeted at humans and agents new to the repository. Must include:
- What the daemon does and why it exists
- Prerequisites and quickstart (build, run, connect an agent)
- Configuration reference (env vars, flags)
- How to contribute (branch strategy, PR checklist, where specs live)
- Links to `documents/` files for deeper reading

### Update obligation

When making an architecturally significant change, update the relevant documents in the same PR/commit as the code change. A change is architecturally significant if it:
- Adds, removes, or renames a Clean Architecture layer or major package
- Introduces or removes an external dependency (DB, vector store, LLM provider)
- Changes the OKF domain model (`Node`, `Edge`, `Facet`, `Graph`)
- Adds, removes, or changes the contract of an MCP tool
- Changes deployment, configuration, or operational topology
- Records a new architectural decision (always requires an ADR entry)
