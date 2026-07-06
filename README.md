# tahu

`tahu` is an OKF (Open Knowledge Format v0.1) knowledge-management daemon for AI agents. It manages hierarchical markdown knowledge bases and exposes 14 structured tools over MCP (Model Context Protocol) — all search runs in-process with zero external service dependencies.

---

## Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | ≥ 1.26 | [go.dev/dl](https://go.dev/dl) |
| golangci-lint | latest | `brew install golangci-lint` or [golangci-lint.run](https://golangci-lint.run/usage/install/) |
| goimports | latest | `go install golang.org/x/tools/cmd/goimports@latest` |

---

## Quickstart

### Build

```bash
make build          # produces bin/tahu
```

### Serve — stdio mode (Claude Code, aider, and other CLI agents)

```bash
make run
# or
bin/tahu serve --transport stdio
```

### Serve — HTTP/SSE mode (LangGraph, custom orchestration pipelines)

```bash
make run-http
# or
bin/tahu serve --transport http --port 3000
```

### Connect Claude Code

Add to your Claude Code MCP configuration (`~/.claude/claude_desktop_config.json` or equivalent):

```json
{
  "mcpServers": {
    "tahu": {
      "command": "/path/to/bin/tahu",
      "args": ["serve", "--transport", "stdio"]
    }
  }
}
```

---

## Bundle management

```bash
# Register a bundle
bin/tahu bundle add /path/to/my-okf-bundle --alias my-kb

# List all registered bundles
bin/tahu bundle list

# Force a full re-index after bulk external edits
bin/tahu bundle reindex my-kb
```

---

## Search

```bash
# RAG search — returns ranked chunks ready for LLM synthesis
bin/tahu search "billing retry logic"

# Scope to a specific bundle
bin/tahu search "deploy steps" --scope bundle:my-kb

# Scope to a sub-path
bin/tahu search "authentication" --scope path:my-kb:security

# Adjust result count
bin/tahu search "payment flow" --top-k 10
```

---

## Read a concept

```bash
bin/tahu concept read my-kb:runbooks/deploy-pipeline.md
```

---

## 14 MCP Tools

| Tool | Description |
|---|---|
| `bundle_list` | List all registered bundles with alias, path, concept count, last-indexed timestamp |
| `bundle_add` | Register a bundle by filesystem path and alias |
| `bundle_remove` | Unregister a bundle by alias (files not deleted) |
| `bundle_reindex` | Force full re-embed and reindex of a bundle |
| `concept_read` | Return parsed frontmatter and body for a concept |
| `concept_write` | Create or update an OKF concept document |
| `concept_list` | List all non-reserved `.md` files at a directory level |
| `concept_links` | Return all outbound markdown hyperlinks from a concept |
| `concept_type_list` | Return all distinct frontmatter `type` values in a bundle |
| `index_read` | Return raw content of `index.md` at a directory level |
| `log_read` | Return raw content of `log.md` at a directory level |
| `search_semantic` | Vector similarity search (BM25 in v0.1) |
| `search_keyword` | Okapi BM25 keyword search |
| `search_rag` | Semantic search filtered by minimum score, scoped for LLM synthesis |

See [`user-docs/mcp-tools.md`](user-docs/mcp-tools.md) for full parameter reference and examples.

---

## Configuration

Config is read from `~/.tahu/config.yaml` (overridden by env vars, then CLI flags).

```yaml
transport: stdio              # stdio | http
port: 3000                    # HTTP mode port
bind_addr: 127.0.0.1         # HTTP mode bind address
bundle_registry: ~/.tahu/registry.yaml
embedding_model: bm25         # bm25 (default, v0.1 only)
embedding_batch_size: 64
hnsw_ef_construction: 200
hnsw_m: 16
log_level: info               # debug | info | warn | error
```

| Key | Env var | Default |
|---|---|---|
| `transport` | `TAHU_TRANSPORT` | `stdio` |
| `port` | `TAHU_PORT` | `3000` |
| `bundle_registry` | `TAHU_REGISTRY` | `~/.tahu/registry.yaml` |
| `embedding_model` | `TAHU_EMBED_MODEL` | `bm25` |
| `log_level` | `TAHU_LOG_LEVEL` | `info` |

See [`user-docs/configuration.md`](user-docs/configuration.md) for the full reference.

---

## Project layout

```
cmd/tahu/               entry point — wires all layers, starts server
internal/domain/        entities, interfaces, domain errors (no external deps)
internal/usecase/       application logic (depends only on domain)
internal/adapter/
  mcp/                  14 MCP tool handlers
  okf/                  OKF parser, linker, validator, BundlePathResolver, FileNodeRepository
  embedder/             BM25Embedder (pure-Go), chunker
  vectorstore/          HNSWStore (coder/hnsw, disk-backed)
  llm/                  reserved — no implementations in v0.1
internal/infra/
  config/               YAML config loader + env-var overlay
  registry/             YAMLBundleRepository
  transport/            stdio + HTTP/SSE MCP server wiring
pkg/                    reserved — pkg/okfcodec deferred to future release
spec/                   SDD feature specs (source of truth before code)
specs/                  dev-flow spec directories (YYMMDD-prefixed)
documents/              living product and architecture documentation
user-docs/              end-user guides
```

---

## Contributing

1. Read [`AGENTS.md`](AGENTS.md) — all process, architecture, and coding conventions.
2. Features follow **SDD → TDD**: write or update the spec in `spec/` before writing code.
3. Run `make test-race lint` before opening a PR.
4. Every PR with an architecturally significant change must update `documents/arch-decisions-record.md` and affected `documents/` files.

---

## Documentation

| Document | Purpose |
|---|---|
| [`documents/product-summary.md`](documents/product-summary.md) | One-page executive overview |
| [`documents/product-details.md`](documents/product-details.md) | User stories, capability map, integration points |
| [`documents/technical-details.md`](documents/technical-details.md) | Architecture, data flows, configuration reference |
| [`documents/arch-decisions-record.md`](documents/arch-decisions-record.md) | Append-only ADR log |
| [`user-docs/getting-started.md`](user-docs/getting-started.md) | Step-by-step guide for new users |
| [`user-docs/configuration.md`](user-docs/configuration.md) | Complete configuration reference |
| [`user-docs/mcp-tools.md`](user-docs/mcp-tools.md) | Full reference for all 14 MCP tools |
| [`user-docs/cli-reference.md`](user-docs/cli-reference.md) | CLI subcommand reference |
