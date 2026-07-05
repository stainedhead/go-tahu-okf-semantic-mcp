# Product Details

> Full product context: user stories, capability map, integration points, non-goals. Update whenever a capability is added, removed, or significantly changed.

## User stories

### Bundle management
- As an operator, I can register an OKF bundle by filesystem path and give it an alias, so agents can reference it by name.
- As an operator, I can list all registered bundles with their concept counts and last-indexed timestamps.
- As an operator, I can force a full re-index of a bundle after bulk external edits.

### Knowledge navigation
- As an agent, I can read any concept document given a `bundle-alias:relative/path.md` reference.
- As an agent, I can list all concepts at any directory level of a bundle.
- As an agent, I can read the `index.md` or `log.md` for any directory level.
- As an agent, I can retrieve the outbound link graph of a concept to traverse relationships hop-by-hop.

### Knowledge authoring
- As an agent, I can create or update a concept document; the daemon validates OKF conformance before writing.
- As an agent, writes automatically update `index.md` and append to `log.md` for the affected directory.

### Search and retrieval
- As an agent, I can run a semantic similarity search scoped to all bundles, one bundle, or a sub-directory.
- As an agent, I can run a BM25 keyword search over the same scopes.
- As an agent, I can run a RAG query and receive the top-K ranked chunks with source attribution, ready to pass to an LLM for synthesis.

## Capability map

| Capability | MCP tool(s) | CLI command |
|---|---|---|
| Bundle registry | `bundle_list`, `bundle_add`, `bundle_remove`, `bundle_reindex` | `tahu bundle *` |
| Concept read | `concept_read`, `concept_list`, `concept_links` | `tahu concept read` |
| Concept write | `concept_write` | — |
| Reserved files | `index_read`, `log_read` | — |
| Semantic search | `search_semantic` | `tahu search` |
| Keyword search | `search_keyword` | `tahu search --mode bm25` |
| RAG retrieval | `search_rag` | `tahu search --rag` |
| Type inventory | `concept_type_list` | — |

## Integration points

| Integration | How |
|---|---|
| Claude Code | MCP stdio — add `tahu` to `mcpServers` in Claude Code config |
| LangGraph / LangChain | MCP HTTP/SSE — connect to `http://localhost:3000` |
| Custom Go agents | Import `pkg/` utilities directly if embedding as a library |
| OKF bundles on disk | Any directory tree conforming to OKF v0.1 spec |
| Git | Bundles are plain directories; version control is external (git) |

## Non-goals (reiterated)

See PRD §5 for the canonical list. Short form: no web UI, no cloud hosting, no LLM synthesis, no non-OKF formats, no typed semantic edges, no Windows (v0.1).

---

*For technical implementation details see [`technical-details.md`](technical-details.md).*
