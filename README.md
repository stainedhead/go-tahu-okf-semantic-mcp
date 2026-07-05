# go-tahu-okf-semantic-mcp

An OKF-based knowledge management daemon that exposes an MCP (Model Context Protocol) server for AI agents. Agents use it to read, write, navigate, and semantically search one or more [OKF](https://okf.md) knowledge bases — entirely without external service dependencies.

See [`documents/product-summary.md`](documents/product-summary.md) for a one-page overview and [`go-tahu-okf-semantic-mcp-PRD.md`](go-tahu-okf-semantic-mcp-PRD.md) for the full product spec.

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | ≥ 1.26 | `go version` to verify |
| golangci-lint | latest | `brew install golangci-lint` or [docs](https://golangci-lint.run/usage/install/) |
| goimports | latest | `go install golang.org/x/tools/cmd/goimports@latest` |

---

## Build

```bash
make build          # produces bin/tahu
make test-race      # unit tests with race detector
make lint           # golangci-lint
make cover          # coverage report in browser
```

Run a single test:

```bash
go test -run TestName ./internal/usecase/...
```

Run integration tests (require external deps tagged):

```bash
make test-integration
```

---

## Run

### stdio mode (CLI agents — Claude Code, aider)

```bash
make run
# or
bin/tahu serve --transport stdio
```

Add to your MCP client config:

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

### HTTP/SSE mode (orchestration agents — LangGraph, custom pipelines)

```bash
make run-http
# or
bin/tahu serve --transport http --port 3000
```

---

## Bundle management

```bash
bin/tahu bundle list
bin/tahu bundle add /path/to/my-okf-bundle --alias my-kb
bin/tahu bundle reindex my-kb
bin/tahu concept read my-kb:runbooks/deploy-pipeline.md
bin/tahu search "billing retry logic"
```

---

## Configuration

Config is read from `~/.tahu/config.yaml` (overridden by env vars, then CLI flags).

```yaml
transport: stdio
port: 3000
bundle_registry: ~/.tahu/bundles.yaml
embedding:
  model: minilm-l6-v2   # or "bm25" for pure-Go, zero-CGo fallback
  batch_size: 32
log_level: info
```

See [`documents/technical-details.md`](documents/technical-details.md) for the full configuration reference.

---

## Project layout

```
cmd/tahu/           entry point — wires all layers, starts server
internal/domain/    entities, interfaces, domain errors (no external deps)
internal/usecase/   application logic (depends only on domain)
internal/adapter/   interface implementations (MCP handlers, embedder, vector store, LLM)
internal/infra/     transport, DB drivers, third-party SDK wiring
pkg/                exportable utilities (OKF codec, embedding helpers)
spec/               feature specs — SDD source of truth
documents/          living product and architecture documentation
user-docs/          end-user guides
```

---

## Contributing

1. Read [`AGENTS.md`](AGENTS.md) — all process, architecture, and coding conventions live there.
2. Features follow **SDD → TDD**: write or update the spec in `spec/` before writing code.
3. Every PR that contains an architecturally significant change must update `documents/arch-decisions-record.md` and any affected `documents/` files.
4. Run `make test-race lint` before opening a PR.

See [`documents/product-details.md`](documents/product-details.md) for capability map and integration points.
