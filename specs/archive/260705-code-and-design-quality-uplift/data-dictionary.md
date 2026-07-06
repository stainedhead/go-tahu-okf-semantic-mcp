# Data Dictionary: Code & Design Quality Uplift

**Feature:** code-and-design-quality-uplift
**Created:** 2026-07-05

---

## Purpose

Documents new and modified data structures introduced by this uplift. Existing types are referenced by their current location; only changes or additions are documented here.

---

## New / Modified Types

### `domain.ConceptRef` (modified — FR-029)

Currently a plain struct with no validation. Post-uplift:

```go
// NewConceptRef returns a validated ConceptRef or an error.
// Constraints: BundleAlias non-empty, no ':', RelativePath non-empty, ends in ".md", no "..".
func NewConceptRef(bundleAlias, relativePath string) (ConceptRef, error)

// Validate returns an error if the ref violates any invariant.
func (r ConceptRef) Validate() error
```

### `domain.OKFFrontmatter` (modified — FR-030)

```go
// Validate returns ErrMissingType if Type is empty.
func (fm OKFFrontmatter) Validate() error
```

### `domain.Clock` (new — FR-031)

```go
// Clock is an injectable time source used by use cases that record timestamps.
type Clock func() time.Time
```

Used by: `BundleService`, `ConceptService.appendLog`.

### `domaintest` package (new — FR-032)

Location: `internal/domain/domaintest/fakes.go`

Exports the shared fake implementations currently duplicated across `usecase` and `mcp` test packages:

```go
package domaintest

type FakeNodeRepository struct { ... }
type FakeBundleRepository struct { ... }
type FakeEmbedder struct { ... }
type FakeVectorStore struct { ... }
```

---

## Modified Interfaces

### `domain.NodeRepository` — no signature change

`ReadReserved`/`WriteReserved`/`List` gain containment enforcement internally (FR-002/003); callers see no interface change.

---

## New Internal Types

### `okf.BundlePathResolver` (new — FR-001)

Location: `internal/adapter/okf/pathresolver.go`

```go
// BundlePathResolver validates and resolves (bundleAlias, relPath) to an absolute path.
type BundlePathResolver struct {
    roots map[string]string // alias → absolute root
}

// Resolve returns a validated absolute path, or an error (ErrNotFound, ErrPathEscape).
func (r *BundlePathResolver) Resolve(bundleAlias, relPath string) (string, error)

// ResolveReserved is like Resolve but skips the reserved-filename check.
func (r *BundlePathResolver) ResolveReserved(bundleAlias, relPath string) (string, error)
```

### `registry.FileLock` (new — FR-028)

Location: `internal/infra/registry/flock.go`

```go
// FileLock is a cross-process advisory file lock backed by syscall.Flock.
type FileLock struct { ... }

func NewFileLock(path string) *FileLock
func (l *FileLock) Lock() error
func (l *FileLock) Unlock() error
```

---

## Configuration Changes

### `config.Config` (modified — FR-015/016/028)

| Field | Change |
|---|---|
| `EmbeddingModel` | Unknown value → startup error (was silent BM25 fallback) |
| `EmbeddingBatchSize` | Validated > 0 (was unvalidated) |
| `Port` | Invalid value → error at load time (was silently ignored) |
| `Transport` | Unknown value → error at load time (was silent default) |
| `LogLevel` | Unknown value → error at load time (was silent default) |

`config.Load(path string)` signature change: accepts an explicit path (previously always read `~/.tahu/config.yaml`).

---

## Enumerations

### Embedding model values (FR-015)

| Value | Behavior |
|---|---|
| `"bm25"` | BM25Embedder (only currently supported value) |
| any other | Startup error: "unsupported embedding model: <value>" |

---

## API / CLI Changes

| Command | Change |
|---|---|
| `tahu serve --config <path>` | Now actually reads the specified file |
| `tahu serve --transport http --bind <non-loopback>` | Returns startup error |
| `TAHU_EMBED_MODEL=<unsupported>` | Returns startup error |
| `TAHU_PORT=<invalid>` | Returns startup error |
