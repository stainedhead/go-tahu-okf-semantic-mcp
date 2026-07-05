# Data Dictionary: go-tahu-okf-semantic-mcp-auto-review

**Feature:** go-tahu-okf-semantic-mcp-auto-review  
**Date:** 2026-07-05

These fixes do not introduce new domain entities. They modify existing behavior and add infrastructure for concurrency control and context propagation.

---

## Modified Interfaces

### `domain.NodeRepository` (possible extension for FR-004)

If the bundle-scoped mutex approach requires a new repository method for atomic log append, the interface may gain:

```go
// AppendReserved atomically appends content to a reserved file path,
// creating the file if it does not exist.
AppendReserved(ctx context.Context, bundleAlias, relPath, content string) error
```

This is optional — the bundle-scoped mutex approach does not require an interface change.

---

## New Values / Constants

### `transport.contextKey` (FR-005)

```go
type contextKey string
const requestIDKey contextKey = "request_id"
```

Unexported type to prevent key collisions between packages.

### `transport.RequestIDFromContext` (FR-005)

```go
func RequestIDFromContext(ctx context.Context) string
```

Returns the request_id stored by `loggingMiddleware`, or `""` if not present.

---

## Concurrency Model Change (FR-002 + FR-004)

**Before fix:**
- `FileNodeRepository.mu` (single `sync.RWMutex`) serializes all `Put` calls across all bundles
- `appendLog` (use case) operates outside the mutex: Read → concat → Write

**After fix (bundle-scoped mutex approach):**
- `FileNodeRepository.mu` continues to serialize `Put` calls
- `ConceptService` gains a per-bundle advisory lock (e.g., `sync.Map` of `sync.Mutex`) keyed by bundle alias
- The full `WriteConcept` flow (Put + regenerateIndex + appendLog) is wrapped in the advisory lock
- Result: concurrent writes to different bundles proceed in parallel; writes to the same bundle are serialized

---

## Index Table Format Change (FR-003)

**Before fix:** `| Path | Type |` — Type column always empty

**After fix:** `| File | Type | Title |` — Type and Title populated from frontmatter

```markdown
# Index

| File | Type | Title |
|---|---|---|
| concept-a.md | note | Concept A Title |
| concept-b.md | reference | Concept B Title |
```

This is a non-breaking change to the generated `index.md` content format — consistent with the adapter's existing `GenerateIndex` output.

---

## Enumerations

_[No new enumerations]_

## API Request / Response Types

_[No MCP tool schema changes — all fixes are internal behavior corrections]_
