# MCP Tools Reference

tahu exposes 14 MCP tools over stdio or HTTP/SSE. All tools accept and return JSON. Field names in JSON responses match Go struct field names (capitalized).

---

## Bundle management

### bundle_list

List all registered OKF bundles.

**Parameters:** none

**Output:** JSON array of bundle objects.

**Example call:**
```json
{
  "tool": "bundle_list"
}
```

**Example response:**
```json
[
  {
    "Alias": "my-kb",
    "RootPath": "/Users/you/my-kb",
    "Description": "My knowledge base",
    "Tags": ["personal"],
    "CreatedAt": "2026-07-05T10:00:00Z",
    "LastIndexedAt": "2026-07-05T11:00:00Z",
    "ConceptCount": 42
  }
]
```

**Errors:** none (empty array returned if no bundles registered).

---

### bundle_add

Register an OKF bundle by filesystem path and alias.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `alias` | string | yes | Unique identifier for the bundle (max 4 KB) |
| `path` | string | yes | Absolute filesystem path to the bundle root directory |
| `description` | string | no | Human-readable description |
| `tags` | string[] | no | Optional tags |

**Output:** JSON object of the created bundle entry.

**Example call:**
```json
{
  "tool": "bundle_add",
  "alias": "my-kb",
  "path": "/Users/you/my-kb",
  "description": "Personal knowledge base",
  "tags": ["personal", "work"]
}
```

**Example response:**
```json
{
  "Alias": "my-kb",
  "RootPath": "/Users/you/my-kb",
  "Description": "Personal knowledge base",
  "Tags": ["personal", "work"],
  "CreatedAt": "2026-07-05T10:00:00Z",
  "LastIndexedAt": "0001-01-01T00:00:00Z",
  "ConceptCount": 0
}
```

**Errors:**
- Path does not exist on disk
- Path contains no `.md` files
- Alias or path already registered

---

### bundle_remove

Unregister a bundle by alias. Files on disk are not deleted.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `alias` | string | yes | Alias of the bundle to remove |

**Output:** Text confirmation.

**Example call:**
```json
{
  "tool": "bundle_remove",
  "alias": "my-kb"
}
```

**Example response:**
```
bundle removed: my-kb
```

**Errors:**
- Alias not found

---

### bundle_reindex

Force a full re-embed and reindex of a bundle. Updates `LastIndexedAt`.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `alias` | string | yes | Alias of the bundle to reindex |

**Output:** Text confirmation.

**Example call:**
```json
{
  "tool": "bundle_reindex",
  "alias": "my-kb"
}
```

**Example response:**
```
bundle reindexed: my-kb
```

**Errors:**
- Alias not found

---

## OKF read / navigate

### concept_read

Return parsed frontmatter and markdown body for an OKF concept.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `ref` | string | yes | Concept reference: `alias:relative/path.md` |

**Output:** JSON object of the parsed concept.

**Example call:**
```json
{
  "tool": "concept_read",
  "ref": "my-kb:notes/deployment.md"
}
```

**Example response:**
```json
{
  "Ref": {
    "BundleAlias": "my-kb",
    "RelativePath": "notes/deployment.md"
  },
  "Frontmatter": {
    "Type": "runbook",
    "Title": "Deployment Runbook",
    "Description": "Steps to deploy the production service",
    "Resource": "",
    "Tags": ["devops"],
    "Timestamp": "2026-07-05T10:00:00Z",
    "Extra": {}
  },
  "Body": "# Deployment Runbook\n\n1. Run smoke tests.\n2. Push to production.",
  "OutboundLinks": [
    {
      "Target": "../apis/payments-api.md",
      "Text": "Payments API",
      "Broken": false
    }
  ]
}
```

**Errors:**
- Concept not found (structured not-found error)
- Invalid ref format

---

### concept_write

Create or update an OKF concept document. Regenerates `index.md` and appends to `log.md` in the containing directory on success.

**Parameters:**

| Name | Type | Required | Constraints | Description |
|---|---|---|---|---|
| `ref` | string | yes | Must not target `index.md` or `log.md` | Concept reference: `alias:relative/path.md` |
| `type` | string | yes | Non-empty, max 4 KB | OKF frontmatter `type` field |
| `title` | string | no | Max 4 KB | Frontmatter title |
| `description` | string | no | Max 4 KB | Frontmatter description |
| `resource` | string | no | Max 4 KB | Frontmatter resource URI |
| `tags` | string[] | no | | Frontmatter tags |
| `body` | string | no | Max 1 MB | Markdown body content |

**Output:** Text confirmation.

**Example call:**
```json
{
  "tool": "concept_write",
  "ref": "my-kb:notes/deployment.md",
  "type": "runbook",
  "title": "Deployment Runbook",
  "description": "Steps to deploy the production service",
  "tags": ["devops"],
  "body": "# Deployment Runbook\n\n1. Run smoke tests.\n2. Push to production."
}
```

**Example response:**
```
concept written: my-kb:notes/deployment.md
```

**Errors:**
- `type` is empty
- `ref` targets `index.md` or `log.md`
- Body exceeds 1 MB
- Bundle alias not registered
- Invalid ref format

---

### concept_list

List all non-reserved `.md` files at a given directory level within a bundle.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `bundle_alias` | string | yes | Alias of the bundle |
| `sub_path` | string | no | Sub-directory within the bundle. Empty = bundle root |

**Output:** JSON array of concept ref objects.

**Example call:**
```json
{
  "tool": "concept_list",
  "bundle_alias": "my-kb",
  "sub_path": "notes"
}
```

**Example response:**
```json
[
  {
    "BundleAlias": "my-kb",
    "RelativePath": "notes/deployment.md"
  },
  {
    "BundleAlias": "my-kb",
    "RelativePath": "notes/hello.md"
  }
]
```

**Errors:**
- Bundle alias not registered

*Non-existent `sub_path` returns an empty array, not an error.*

---

### concept_links

Return all outbound markdown hyperlink targets from a concept body. Broken links (targets that do not exist on disk) are included with `Broken: true`.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `ref` | string | yes | Concept reference: `alias:relative/path.md` |

**Output:** JSON array of link objects.

**Example call:**
```json
{
  "tool": "concept_links",
  "ref": "my-kb:notes/deployment.md"
}
```

**Example response:**
```json
[
  {
    "Target": "../apis/payments-api.md",
    "Text": "Payments API",
    "Broken": false
  },
  {
    "Target": "../services/missing.md",
    "Text": "Missing Doc",
    "Broken": true
  }
]
```

**Errors:**
- Concept not found
- Invalid ref format

---

### concept_type_list

Return all distinct frontmatter `type` values present in a bundle.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `bundle_alias` | string | yes | Alias of the bundle |

**Output:** JSON array of strings.

**Example call:**
```json
{
  "tool": "concept_type_list",
  "bundle_alias": "my-kb"
}
```

**Example response:**
```json
["note", "runbook", "api", "decision"]
```

**Errors:**
- Bundle alias not registered

---

### index_read

Return the raw content of `index.md` at a given directory level.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `bundle_alias` | string | yes | Alias of the bundle |
| `dir_path` | string | no | Directory path within the bundle. Empty = bundle root |

**Output:** Raw markdown text.

**Example call:**
```json
{
  "tool": "index_read",
  "bundle_alias": "my-kb",
  "dir_path": "notes"
}
```

**Example response:**
```
# notes

- [Deployment Runbook](deployment.md)
- [Hello World](hello.md)
```

**Errors:**
- `index.md` not found at the given path (structured not-found error)

---

### log_read

Return the raw content of `log.md` at a given directory level.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `bundle_alias` | string | yes | Alias of the bundle |
| `dir_path` | string | no | Directory path within the bundle. Empty = bundle root |

**Output:** Raw markdown text.

**Example call:**
```json
{
  "tool": "log_read",
  "bundle_alias": "my-kb",
  "dir_path": "notes"
}
```

**Example response:**
```
# Change Log

- 2026-07-05T10:00:00Z — wrote notes/deployment.md
- 2026-07-05T09:00:00Z — wrote notes/hello.md
```

**Errors:**
- `log.md` not found at the given path (structured not-found error)

---

## Search

All three search tools share the same scope format and return the same `ScoredChunk` shape.

### Scope format

| Value | Meaning |
|---|---|
| `global` (default) | Search across all registered bundles |
| `bundle:<alias>` | Restrict to one bundle |
| `path:<alias>:<subpath>` | Restrict to a sub-path within a bundle |

### ScoredChunk shape

```json
{
  "Source": "my-kb:notes/deployment.md",
  "ChunkIndex": 0,
  "ChunkText": "Run smoke tests. Push to production.",
  "Score": 0.87,
  "FrontmatterSummary": "runbook:Deployment Runbook"
}
```

---

### search_semantic

Return a ranked list of chunks via vector similarity search. In v0.1, the underlying embedding is BM25.

**Parameters:**

| Name | Type | Required | Constraints | Description |
|---|---|---|---|---|
| `query` | string | yes | Max 4 KB | Natural-language search query |
| `scope` | string | no | | Search scope (default: `global`) |
| `top_k` | int | no | ≥ 1 | Maximum results to return (default: 10) |

**Output:** JSON array of `ScoredChunk` objects in descending score order.

**Example call:**
```json
{
  "tool": "search_semantic",
  "query": "how to deploy the service",
  "scope": "bundle:my-kb",
  "top_k": 5
}
```

**Example response:**
```json
[
  {
    "Source": "my-kb:notes/deployment.md",
    "ChunkIndex": 0,
    "ChunkText": "Run smoke tests. Push to production. Monitor for 10 minutes.",
    "Score": 0.91,
    "FrontmatterSummary": "runbook:Deployment Runbook"
  }
]
```

**Errors:**
- Invalid scope format

---

### search_keyword

Return a ranked list of chunks via Okapi BM25 keyword search. Same scope semantics and response shape as `search_semantic`.

**Parameters:**

| Name | Type | Required | Constraints | Description |
|---|---|---|---|---|
| `query` | string | yes | Max 4 KB | Keyword search query |
| `scope` | string | no | | Search scope (default: `global`) |
| `top_k` | int | no | ≥ 1 | Maximum results to return (default: 10) |

**Output:** JSON array of `ScoredChunk` objects in descending score order.

**Example call:**
```json
{
  "tool": "search_keyword",
  "query": "smoke tests deployment",
  "scope": "global",
  "top_k": 3
}
```

**Errors:**
- Invalid scope format

---

### search_rag

Semantic search with a minimum score filter. Returns chunks for LLM synthesis — the agent is responsible for passing the chunks to an LLM.

**Parameters:**

| Name | Type | Required | Constraints | Description |
|---|---|---|---|---|
| `query` | string | yes | Max 4 KB | Natural-language retrieval query |
| `scope` | string | no | | Search scope (default: `global`) |
| `top_k` | int | no | 1–20 | Maximum chunks to return (default: 5) |
| `min_score` | float | no | 0.0–1.0 | Minimum similarity score threshold (default: 0.0) |

**Output:** JSON array of `ScoredChunk` objects with `Score >= min_score`, in descending order. Returns an empty array (not an error) when no chunks meet the threshold.

**Example call:**
```json
{
  "tool": "search_rag",
  "query": "payment retry logic",
  "scope": "bundle:my-kb",
  "top_k": 5,
  "min_score": 0.5
}
```

**Example response:**
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

**Errors:**
- Invalid scope format
- `top_k` < 1 or > 20
- `min_score` < 0.0 or > 1.0
