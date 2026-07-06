# Data Dictionary: Code & Design Quality Uplift — Auto-Review Fixes

**Feature:** code-and-design-quality-uplift-auto-review
**Created:** 2026-07-06

No new data structures are introduced. This spec modifies existing implementations.

---

## Modified Interfaces

### `domain.VectorStore` (unchanged — already correct)
```go
type VectorStore interface {
    Upsert(ctx context.Context, chunks []EmbeddingChunk) error
    Search(ctx context.Context, query []float32, scope Scope, topK int) ([]ScoredChunk, error)
    Delete(ctx context.Context, ids []string) error  // domaintest was missing this
    Persist(ctx context.Context) error
    Load(ctx context.Context) error
}
```

### `domaintest.VectorStore` (to be fixed — FR-001)
Replace `DeleteByBundle`/`DeleteByIDs` with `Delete(ctx, ids []string) error` matching the interface.
