# Research: go-tahu-okf-semantic-mcp-auto-review

**Feature:** go-tahu-okf-semantic-mcp-auto-review  
**Date:** 2026-07-05  
**Source PRD:** `specs/260705-go-tahu-okf-semantic-mcp-auto-review/go-tahu-okf-semantic-mcp-auto-review-PRD.md`

---

## Research Questions

### RQ-1 (OQ-001): Which concurrency fix approach for `appendLog`?

Three approaches were identified in the PRD:
1. Bundle-scoped mutex held across the full `WriteConcept` flow (recommended)
2. Append-only `WriteReserved` variant on the `NodeRepository` interface
3. Move log management back into a single locked adapter operation (reintroduces Clean Architecture tension)

**Recommendation from PRD:** Option 1 — bundle-scoped mutex. Preserves Clean Architecture (use case owns index/log), straightforward to reason about. Implementer to confirm before coding.

**Criteria:** Does option 1 interact with the existing per-repo mutex in `FileNodeRepository`? Can a bundle-level advisory mutex be added without risking deadlock?

### RQ-2: What is the right test structure for the concurrent write test?

`TestConcurrentConceptWrite_LogPreservesAllEntries` needs to:
- Spin up N goroutines writing different concepts to the same directory
- Wait for all to complete
- Assert log.md has exactly N entries

Questions: What N value is appropriate? Should the test use `t.Parallel()`? Should it also use `go test -race` in CI explicitly, or trust the race detector via `go test -race ./...`?

### RQ-3: Does `NodeRepository.Get` need to be called per-ref in `regenerateIndex`, or is there a batch approach?

`regenerateIndex` currently only has `[]ConceptRef` from `List`. To populate type+title, it must call `Get` for each ref. For large bundles (500+ concepts), this is N filesystem reads.

Questions: Does `NodeRepository` have or need a `GetMany` / `ListWithFrontmatter` method? Or should v0.1 accept the N+1 pattern given small bundle sizes in this release?

### RQ-4: Does removing `GenerateIndex`/`AppendLog` from `Put` break any integration tests?

The OKF adapter package has integration tests gated with `//go:build integration`. Do any of them depend on `Put` side-effecting the index or log?

### RQ-5: What is the correct `context.WithValue` key type for `request_id`?

Using an unexported struct key type avoids key collisions between packages. The implementation should define:
```go
type contextKey string
const requestIDKey contextKey = "request_id"
```
Is `contextKey` defined in the `transport` package or a shared `middleware` package? Which package is the best owner for `RequestIDFromContext`?

---

## Industry Standards

_[To be populated if needed — these are all internal correctness fixes, no external standards required]_

## Existing Implementations

- Go `sync.RWMutex` / `sync.Mutex` — standard library, no new dependency
- `context.WithValue` — standard library, Go 1.7+
- Bundle-scoped mutex pattern: common in Go key-value stores (e.g., `bbolt` uses per-bucket locking)

## API Documentation

- `sync.Mutex` GoDoc: https://pkg.go.dev/sync#Mutex
- `context.WithValue` GoDoc: https://pkg.go.dev/context#WithValue
- `go test -race` docs: https://go.dev/doc/articles/race_detector

## Best Practices

- Use unexported `contextKey` type (not `string`) for context keys to avoid collisions
- Bundle-scoped mutex: key on bundle alias (string), store in a `sync.Map` or per-repo map protected by a read mutex
- Table-driven tests for handler coverage (FIX-007): one table, N rows, shared setup

## Open Questions

| ID | Question | Status |
|---|---|---|
| OQ-001 | Which concurrency approach for FIX-004? | Recommendation: bundle-scoped mutex (option 1) |

## References

_[Add as needed during implementation]_
