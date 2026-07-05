# Data Dictionary: go-tahu-okf-semantic-mcp

**Feature:** go-tahu-okf-semantic-mcp  
**Date:** 2026-07-05  
**Status:** Draft — to be refined during research phase

---

## Domain Entities

### BundleEntry
```go
type BundleEntry struct {
    Alias          string    // user-assigned unique identifier
    RootPath       string    // absolute, canonicalized filesystem path
    Description    string
    Tags           []string
    CreatedAt      time.Time
    LastIndexedAt  time.Time
}
```

### OKFFrontmatter
```go
type OKFFrontmatter struct {
    Type        string            // REQUIRED per OKF v0.1
    Title       string
    Description string
    Resource    string            // URI of the described resource
    Tags        []string
    Timestamp   time.Time
    Extra       map[string]any    // unknown keys preserved in insertion order
}
```

### OKFConcept
```go
type OKFConcept struct {
    Ref          ConceptRef
    Frontmatter  OKFFrontmatter
    Body         string        // raw markdown body (after frontmatter block)
    OutboundLinks []ConceptLink
}
```

### ConceptLink
```go
type ConceptLink struct {
    Target string // resolved relative path (e.g. "../apis/payments-api.md")
    Text   string // link display text
    Broken bool   // true if target does not exist on disk
}
```

---

## Value Objects

### ConceptRef
```go
type ConceptRef struct {
    BundleAlias  string // e.g. "my-kb"
    RelativePath string // e.g. "runbooks/deploy-pipeline.md"
}
// String() returns "alias:relative/path.md"
```

### Scope
```go
type Scope struct {
    Kind        ScopeKind // Global | Bundle | Path
    BundleAlias string    // set for Bundle and Path kinds
    SubPath     string    // set for Path kind only
}

type ScopeKind int
const (
    ScopeGlobal ScopeKind = iota
    ScopeBundle
    ScopePath
)
```

---

## Embedding / Search Types

### EmbeddingChunk
```go
type EmbeddingChunk struct {
    ID                  string    // "alias:path:chunk_index"
    BundleAlias         string
    ConceptPath         string
    ChunkIndex          int
    Text                string
    Embedding           []float32
    FrontmatterSummary  string    // "type:title" for display
}
```

### ScoredChunk
```go
type ScoredChunk struct {
    Source             string  // "alias:path"
    ChunkIndex         int
    ChunkText          string
    Score              float32
    FrontmatterSummary string
}
```

---

## Interfaces (defined in `internal/domain/`)

### Embedder
```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dims() int // embedding dimensionality
}
```

### VectorStore
```go
type VectorStore interface {
    Upsert(ctx context.Context, chunks []EmbeddingChunk) error
    Search(ctx context.Context, query []float32, scope Scope, topK int) ([]ScoredChunk, error)
    Delete(ctx context.Context, ids []string) error
    Persist(ctx context.Context) error
    Load(ctx context.Context) error
}
```

### NodeRepository
```go
type NodeRepository interface {
    Get(ctx context.Context, ref ConceptRef) (*OKFConcept, error)
    Put(ctx context.Context, ref ConceptRef, concept *OKFConcept) error
    List(ctx context.Context, bundleAlias string, subPath string) ([]ConceptRef, error)
    ListTypes(ctx context.Context, bundleAlias string) ([]string, error)
}
```

### BundleRepository
```go
type BundleRepository interface {
    Get(ctx context.Context, alias string) (*BundleEntry, error)
    Put(ctx context.Context, entry BundleEntry) error
    Delete(ctx context.Context, alias string) error
    List(ctx context.Context) ([]BundleEntry, error)
}
```

---

## MCP Tool Input / Output Schemas

_[To be fully defined in `internal/adapter/mcp/schema.go` — JSON Schema per tool]_

### search_rag input
```json
{
  "query":     "string (required, max 4KB)",
  "scope":     "string (required: 'global' | 'bundle:<alias>' | 'path:<alias>:<path>')",
  "top_k":     "integer (optional, default 5, max 20)",
  "min_score": "number (optional, default 0.0, range 0.0–1.0)"
}
```

### search_rag output
```json
{
  "chunks": [
    {
      "source":      "string (alias:path)",
      "chunk_index": "integer",
      "chunk_text":  "string",
      "score":       "number"
    }
  ]
}
```

---

## Configuration Types

```go
type Config struct {
    Transport      string         // "stdio" | "http"
    Port           int            // HTTP mode only
    BindAddr       string         // default "127.0.0.1"
    BundleRegistry string         // path to bundles.yaml
    Embedding      EmbeddingConfig
    Index          IndexConfig
    LogLevel       string
}

type EmbeddingConfig struct {
    Model     string // "minilm-l6-v2" | "bm25"
    BatchSize int
}

type IndexConfig struct {
    HNSWEfConstruction int
    HNSWM              int
}
```

---

## Enumerations

```go
// Domain errors (sentinel values)
var (
    ErrNotFound        = errors.New("not found")
    ErrReservedPath    = errors.New("reserved filename")
    ErrMissingType     = errors.New("frontmatter missing required field: type")
    ErrPathEscape      = errors.New("path escapes bundle root")
    ErrInputTooLarge   = errors.New("input exceeds size limit")
    ErrDuplicateAlias  = errors.New("bundle alias already registered")
    ErrDuplicatePath   = errors.New("bundle root path already registered")
)
```
