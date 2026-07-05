# Product Details

> Full product context: user stories, capability map, integration points, non-goals. Update whenever a capability is added, removed, or significantly changed.

## User stories

### Bundle management
- As an operator, I can register an OKF bundle by filesystem path and give it an alias, so agents can reference it by name.
- As an operator, I can list all registered bundles with their concept counts and last-indexed timestamps.
- As an operator, I can remove a bundle from the registry without deleting its files.
- As an operator, I can force a full re-index of a bundle after bulk external edits.

### Knowledge navigation
- As an agent, I can read any concept document given a `bundle-alias:relative/path.md` reference.
- As an agent, I can list all concepts at any directory level of a bundle.
- As an agent, I can read the `index.md` or `log.md` for any directory level.
- As an agent, I can retrieve the outbound link graph of a concept to traverse relationships hop-by-hop.
- As an agent, I can list all distinct frontmatter `type` values present in a bundle.

### Knowledge authoring
- As an agent, I can create or update a concept document; the daemon validates OKF conformance before writing.
- As an agent, writes automatically regenerate `index.md` and append to `log.md` for the affected directory.

### Search and retrieval
- As an agent, I can run a semantic similarity search scoped to all bundles, one bundle, or a sub-directory.
- As an agent, I can run a BM25 keyword search over the same scopes.
- As an agent, I can run a RAG query and receive the top-K ranked chunks (filtered by minimum score) with source attribution, ready to pass to an LLM for synthesis.

## Capability map — all 14 MCP tools

| MCP tool | Description | CLI command |
|---|---|---|
| `bundle_list` | List all registered bundles with alias, path, concept count, and last-indexed timestamp | `tahu bundle list` |
| `bundle_add` | Register a bundle by filesystem path and alias | `tahu bundle add <path> --alias <alias>` |
| `bundle_remove` | Unregister a bundle by alias (files not deleted) | — |
| `bundle_reindex` | Force full re-embed and reindex of a bundle | `tahu bundle reindex <alias>` |
| `concept_read` | Return parsed frontmatter and body for a concept | `tahu concept read <ref>` |
| `concept_write` | Create or update an OKF concept document | — |
| `concept_list` | List all non-reserved `.md` files at a directory level | — |
| `concept_links` | Return all outbound markdown hyperlinks from a concept | — |
| `concept_type_list` | Return all distinct frontmatter `type` values in a bundle | — |
| `index_read` | Return raw content of `index.md` at a directory level | — |
| `log_read` | Return raw content of `log.md` at a directory level | — |
| `search_semantic` | Vector similarity search (BM25 in v0.1) | — |
| `search_keyword` | Okapi BM25 keyword search | `tahu search <query>` |
| `search_rag` | Semantic search filtered by minimum score — retrieval for LLM synthesis | `tahu search <query>` |

## Integration points

| Integration | How |
|---|---|
| Claude Code | Add `tahu serve` to `mcpServers` in Claude Code's MCP config (stdio transport) |
| LangGraph / LangChain | Connect to `http://127.0.0.1:3000` — HTTP/SSE MCP transport |
| Any MCP client | Both transports expose the same 14-tool surface |
| OKF bundles on disk | Any directory tree of markdown files with YAML frontmatter conforming to OKF v0.1 |
| Git | Bundles are plain directories; version control is external |

## Non-goals (v0.1)

- No web UI
- No cloud hosting or cloud API calls
- No LLM synthesis (tahu retrieves; agents synthesize)
- No non-OKF formats (HTML, DOCX, PDF, etc.)
- No typed semantic edges (relationships are plain markdown links)
- No Windows support
- No ONNX embedding tier (deferred; see ADR-006)
- No `pkg/okfcodec` external library (deferred; see ADR-007)

---

*For technical implementation details see [`technical-details.md`](technical-details.md).*
