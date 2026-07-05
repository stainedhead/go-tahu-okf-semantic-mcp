# Research: go-tahu-okf-semantic-mcp

**Feature:** go-tahu-okf-semantic-mcp  
**Date:** 2026-07-05  
**Source PRD:** `specs/260705-go-tahu-okf-semantic-mcp/go-tahu-okf-semantic-mcp-PRD.md`

---

## Research Questions

Derived from PRD open questions and dependency risks.

### RQ-1 (OQ-1): ONNX model selection
Which sentence-transformer model (ONNX format) offers the best accuracy/size tradeoff for short OKF concept prose (frontmatter + 1–3 paragraphs)?  
Candidates: `all-MiniLM-L6-v2` (~22 MB, 384-dim), `all-MiniLM-L12-v2` (~34 MB, 384-dim), `paraphrase-MiniLM-L3-v2` (~17 MB).  
**Criteria:** retrieval quality on short markdown text, inference latency < 50 ms/batch-32, binary size impact.

### RQ-2 (OQ-3): BM25 as zero-CGo floor
Is pure-Go BM25 retrieval quality sufficient for the "no external deps" tier?  
Test: compare BM25 vs MiniLM top-5 recall on a sample OKF bundle with 500+ concepts.  
**Criteria:** if BM25 recall@5 ≥ 0.6, it is acceptable as the zero-CGo fallback.

### RQ-3: MCP Go SDK selection
What is the most appropriate Go MCP SDK for this daemon?  
Candidates: `mark3labs/mcp-go`, `modelcontextprotocol/go-sdk` (official, if available), roll-our-own JSON-RPC 2.0.  
**Criteria:** stdio + HTTP/SSE support, tool registration API, JSON Schema validation, license (Apache 2.0 or MIT preferred), maintenance status.

### RQ-4 (OQ-5): Chunk overlap strategy
What chunk overlap is appropriate for OKF concept prose?  
OKF documents are typically short (< 500 tokens). Overlap between paragraph chunks may hurt precision by duplicating context.  
**Criteria:** benchmark retrieval quality at 0%, 10%, 20% overlap on sample bundle.

### RQ-5: `coder/hnsw` persistence API
Does `github.com/coder/hnsw` support incremental persistence (add/update vectors without full rebuild)?  
Investigate: `Save`/`Load` API, mmap support, index file format, thread safety for concurrent reads + serialized writes.  
**Criteria:** must support lazy build on first query and incremental update on `concept_write`.

### RQ-6 (OQ-2): `index.md` auto-generation policy
Should `concept_write` always regenerate `index.md`, or only when one already exists?  
Implications: always-generate ensures navigation is always available; conditional preserves operator intent when `index.md` was deliberately absent.  
**Decision needed before FR-011 implementation.**

---

## Industry Standards

_[To be populated during research phase]_

## Existing Implementations

- `mfdaves/okf-mcp` — reference MCP server for OKF; scope and capabilities unconfirmed
- `kcmd` — optional OKF CLI; MCP support reported
- `W4G1/okf` — Rust implementation; useful for conformance reference

## API Documentation

- OKF SPEC v0.1: `github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md`
- `coder/hnsw` GoDoc: `pkg.go.dev/github.com/coder/hnsw`
- `goldmark` GoDoc: `pkg.go.dev/github.com/yuin/goldmark`
- `yaml.v3` GoDoc: `pkg.go.dev/gopkg.in/yaml.v3`
- MCP spec: `modelcontextprotocol.io`

## Best Practices

_[To be populated during research phase]_

## Open Questions

See PRD §15 (OQ-1 through OQ-5). RQ-3 (MCP SDK) and RQ-5 (HNSW persistence) are blockers for M2.

## References

_[To be populated as research is conducted]_
