# CLI Reference

`tahu` — OKF knowledge-management daemon with MCP tools.

```
Usage:
  tahu [command]

Available Commands:
  serve     Start the MCP server
  bundle    Manage OKF bundle registrations
  search    Run a RAG semantic search over registered bundles
  concept   Read or write OKF concepts
  help      Help about any command

Flags:
  -h, --help   help for tahu
```

---

## tahu serve

Start the tahu MCP server.

```
Usage:
  tahu serve [flags]

Flags:
  --transport string   MCP transport: stdio|http (default "stdio")
  --port int           TCP port for HTTP transport (default from config: 3000)
  --bind string        Bind address for HTTP transport (default from config: 127.0.0.1)
  --config string      Config file path (default: ~/.tahu/config.yaml)
  -h, --help           help for serve
```

**Examples:**

```bash
# stdio mode — default; used by Claude Code and other CLI agents
tahu serve

# Equivalent explicit form
tahu serve --transport stdio

# HTTP/SSE mode for orchestration agents
tahu serve --transport http

# HTTP mode on a custom port and bind address
tahu serve --transport http --port 8080 --bind 0.0.0.0
```

**Notes:**
- In stdio mode the server reads JSON-RPC 2.0 from stdin and writes responses to stdout. Logs go to stderr.
- In HTTP/SSE mode the server binds to `<bind>:<port>` and accepts MCP over HTTP POST and Server-Sent Events.
- The server handles SIGINT and SIGTERM cleanly — the HNSW vector index is flushed to disk before exit.

---

## tahu bundle

Manage OKF bundle registrations.

```
Usage:
  tahu bundle [command]

Available Commands:
  list      List all registered OKF bundles
  add       Register an OKF bundle by filesystem path
  reindex   Force a full re-embed and reindex of a bundle
```

---

### tahu bundle list

List all registered bundles with alias, root path, concept count, and last-indexed timestamp.

```
Usage:
  tahu bundle list
```

**Example:**

```bash
tahu bundle list
```

**Output:**

```
ALIAS    ROOT_PATH               CONCEPT_COUNT  LAST_INDEXED_AT
my-kb    /Users/you/my-kb        42             2026-07-05T11:00:00Z
work-kb  /Users/you/work-notes   17             2026-07-05T10:30:00Z
```

---

### tahu bundle add

Register an OKF bundle by filesystem path and alias.

```
Usage:
  tahu bundle add <path> [flags]

Flags:
  --alias string   Unique alias for the bundle (required)
  -h, --help       help for add
```

**Arguments:**

| Argument | Description |
|---|---|
| `<path>` | Absolute filesystem path to the bundle root directory |

**Examples:**

```bash
# Register a bundle
tahu bundle add /Users/you/my-kb --alias my-kb

# Register with a nested path
tahu bundle add /Users/you/work/projects/docs --alias work-docs
```

**Output:**

```
Bundle registered: alias=my-kb path=/Users/you/my-kb
```

**Errors:**
- Path does not exist
- No `.md` files found at path
- Alias or path already registered

---

### tahu bundle reindex

Force a full re-embed and reindex of a bundle. Use this after bulk external edits to the bundle, or after changing HNSW parameters.

```
Usage:
  tahu bundle reindex <alias>
```

**Arguments:**

| Argument | Description |
|---|---|
| `<alias>` | Alias of the bundle to reindex |

**Example:**

```bash
tahu bundle reindex my-kb
```

**Output:**

```
Bundle reindexed: my-kb
```

---

## tahu search

Run a RAG semantic search over registered bundles. Returns ranked chunks as JSON.

```
Usage:
  tahu search <query> [flags]

Flags:
  --scope string   Search scope: global|bundle:<alias>|path:<alias>:<subpath> (default "global")
  --top-k int      Maximum number of results (default 5)
  -h, --help       help for search
```

**Arguments:**

| Argument | Description |
|---|---|
| `<query>` | Natural-language or keyword search query |

**Examples:**

```bash
# Search across all bundles
tahu search "billing retry logic"

# Scope to a specific bundle
tahu search "deploy steps" --scope bundle:my-kb

# Scope to a sub-path within a bundle
tahu search "authentication" --scope path:my-kb:security

# Return more results
tahu search "payment flow" --top-k 10
```

**Output:** JSON array of `ScoredChunk` objects printed to stdout.

```json
[
  {
    "Source": "my-kb:payments/retry-policy.md",
    "ChunkIndex": 0,
    "ChunkText": "Retry failed payments with exponential backoff: 1s, 2s, 4s, max 3 attempts.",
    "Score": 0.78,
    "FrontmatterSummary": "policy:Payment Retry Policy"
  }
]
```

*"No results." is printed when the search returns no chunks.*

---

## tahu concept

Read or write OKF concepts.

```
Usage:
  tahu concept [command]

Available Commands:
  read      Read and print an OKF concept
```

---

### tahu concept read

Read and print a concept by its reference.

```
Usage:
  tahu concept read <alias:relative/path.md>
```

**Arguments:**

| Argument | Description |
|---|---|
| `<alias:relative/path.md>` | Concept reference in `alias:relative/path.md` format |

**Example:**

```bash
tahu concept read my-kb:runbooks/deploy-pipeline.md
```

**Output:** JSON object of the parsed concept printed to stdout.

```json
{
  "Ref": {
    "BundleAlias": "my-kb",
    "RelativePath": "runbooks/deploy-pipeline.md"
  },
  "Frontmatter": {
    "Type": "runbook",
    "Title": "Deploy Pipeline",
    "Description": "Steps to deploy the service",
    "Resource": "",
    "Tags": ["devops"],
    "Timestamp": "2026-07-05T10:00:00Z",
    "Extra": {}
  },
  "Body": "# Deploy Pipeline\n\n1. Run tests.\n2. Deploy.",
  "OutboundLinks": []
}
```

**Errors:**
- Concept not found
- Invalid ref format (must contain a `:` separator)

---

## Global notes

- All subcommands read configuration from `~/.tahu/config.yaml` and respect environment-variable overrides. See [`configuration.md`](configuration.md) for details.
- Errors are printed to stderr; the process exits with code 1.
- Log output (structured JSON) is written to stderr when running `serve`.
