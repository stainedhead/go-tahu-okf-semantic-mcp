package domain

import "context"

// Embedder converts text slices into dense vector representations.
type Embedder interface {
	// Embed returns one embedding vector per input text, in the same order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dims returns the dimensionality of each embedding vector.
	Dims() int
}

// VectorStore persists and queries embedding chunks.
type VectorStore interface {
	// Upsert inserts or replaces the given chunks by ID.
	Upsert(ctx context.Context, chunks []EmbeddingChunk) error
	// Search returns up to topK chunks nearest to query, filtered by scope.
	Search(ctx context.Context, query []float32, scope Scope, topK int) ([]ScoredChunk, error)
	// Delete removes chunks by their IDs.
	Delete(ctx context.Context, ids []string) error
	// Persist flushes the index to durable storage.
	Persist(ctx context.Context) error
	// Load restores the index from durable storage.
	Load(ctx context.Context) error
}

// NodeRepository provides read/write access to OKF concept documents
// and reserved OKF files (index.md, log.md).
type NodeRepository interface {
	// Get retrieves a parsed concept by its ref.
	Get(ctx context.Context, ref ConceptRef) (*OKFConcept, error)
	// Put creates or replaces a concept document.
	Put(ctx context.Context, ref ConceptRef, concept *OKFConcept) error
	// List returns refs for all non-reserved .md files under subPath within
	// the given bundle. subPath may be empty to list the bundle root.
	List(ctx context.Context, bundleAlias string, subPath string) ([]ConceptRef, error)
	// ListTypes returns all distinct frontmatter type values in a bundle.
	ListTypes(ctx context.Context, bundleAlias string) ([]string, error)
	// ReadReserved returns the raw content of a reserved file (e.g. index.md,
	// log.md) at relPath within the bundle.
	ReadReserved(ctx context.Context, bundleAlias string, relPath string) (string, error)
	// WriteReserved creates or replaces a reserved file at relPath within the
	// bundle.
	WriteReserved(ctx context.Context, bundleAlias string, relPath string, content string) error
}

// BundleRepository provides CRUD access to registered bundle metadata.
type BundleRepository interface {
	// Get retrieves a bundle by alias.
	Get(ctx context.Context, alias string) (*BundleEntry, error)
	// Put creates or replaces a bundle entry.
	Put(ctx context.Context, entry BundleEntry) error
	// Delete removes a bundle entry by alias.
	Delete(ctx context.Context, alias string) error
	// List returns all registered bundles.
	List(ctx context.Context) ([]BundleEntry, error)
}
