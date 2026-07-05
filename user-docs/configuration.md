# Configuration Reference

tahu reads configuration from a YAML file, with optional environment-variable and CLI-flag overrides.

**Precedence (highest to lowest):** CLI flags > environment variables > config file > built-in defaults.

---

## Config file location

```
~/.tahu/config.yaml
```

If the file does not exist, tahu starts with all defaults — no error is raised.

---

## All configuration keys

| YAML key | Env var | Type | Default | Description |
|---|---|---|---|---|
| `transport` | `TAHU_TRANSPORT` | string | `stdio` | MCP transport mode: `stdio` or `http` |
| `port` | `TAHU_PORT` | int | `3000` | TCP port for HTTP transport mode |
| `bind_addr` | — | string | `127.0.0.1` | Bind address for HTTP transport mode |
| `bundle_registry` | `TAHU_REGISTRY` | string | `~/.tahu/registry.yaml` | Path to the bundle registry YAML file |
| `embedding_model` | `TAHU_EMBED_MODEL` | string | `bm25` | Embedding backend: `bm25` or `minilm-l6-v2`* |
| `embedding_batch_size` | — | int | `64` | Number of texts embedded per batch |
| `hnsw_ef_construction` | — | int | `200` | HNSW build-time search depth (higher = better quality, slower build) |
| `hnsw_m` | — | int | `16` | HNSW max connections per node (higher = better recall, more memory) |
| `log_level` | `TAHU_LOG_LEVEL` | string | `info` | Structured log verbosity: `debug`, `info`, `warn`, `error` |

*`minilm-l6-v2` is parsed but not implemented in v0.1; BM25 is always used.

Only five keys can be set via environment variable: `transport`, `port`, `bundle_registry`, `embedding_model`, and `log_level`. The remaining keys (`bind_addr`, `embedding_batch_size`, `hnsw_ef_construction`, `hnsw_m`) are config-file or flag only.

---

## Example: stdio mode (default)

This is the default and requires no config file. To make it explicit:

```yaml
# ~/.tahu/config.yaml
transport: stdio
bundle_registry: ~/.tahu/registry.yaml
embedding_model: bm25
log_level: info
```

Launch with `serve` (stdio is the default transport):

```bash
bin/tahu serve
```

---

## Example: HTTP/SSE mode

For orchestration agents (LangGraph, custom pipelines):

```yaml
# ~/.tahu/config.yaml
transport: http
port: 3000
bind_addr: 127.0.0.1
log_level: info
```

Or override at runtime:

```bash
TAHU_TRANSPORT=http TAHU_PORT=8080 bin/tahu serve
```

Or via CLI flags (highest precedence):

```bash
bin/tahu serve --transport http --port 8080 --bind 0.0.0.0
```

---

## Example: debug logging

```bash
TAHU_LOG_LEVEL=debug bin/tahu serve
```

Logs are structured JSON written to stderr.

---

## Example: custom registry location

If you keep your registry alongside your bundles:

```yaml
bundle_registry: /data/tahu/registry.yaml
```

Or:

```bash
TAHU_REGISTRY=/data/tahu/registry.yaml bin/tahu serve
```

The vector index (`hnsw.index`) is stored in the same directory as `registry.yaml`.

---

## HNSW tuning

For large bundles (> 10k concepts) you may want to increase HNSW parameters:

```yaml
hnsw_ef_construction: 400   # better graph quality during indexing
hnsw_m: 32                  # more connections = better recall, more memory
```

These parameters only affect newly indexed chunks. Run `tahu bundle reindex <alias>` after changing them to rebuild the index.

---

## CLI flag overrides

The `serve` subcommand accepts flags that override the config file and environment:

| Flag | Description |
|---|---|
| `--transport stdio\|http` | MCP transport |
| `--port <int>` | HTTP port (default from config: 3000) |
| `--bind <addr>` | HTTP bind address |
| `--config <path>` | Config file path (currently informational; file is always read from `~/.tahu/config.yaml`) |

Example:

```bash
bin/tahu serve --transport http --port 9000 --bind 0.0.0.0
```
