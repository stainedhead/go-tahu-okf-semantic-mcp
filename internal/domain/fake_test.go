// Package domain_test provides in-memory fakes of all domain interfaces for
// use in unit tests. Fakes are thread-safe via sync.RWMutex.
//
// NOTE: because these types live in a _test.go file they are compiled only
// during test builds and cannot be imported by other packages. Tests in
// internal/usecase/ or elsewhere that need these fakes should either copy
// them locally or move them to a non-test file in a shared testing package
// (e.g. internal/domain/domaintest/).
package domain_test

import (
	"context"
	"strings"
	"sync"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// Compile-time interface conformance checks. If any fake drifts from its
// interface the compiler will fail here, making the mismatch obvious.
var (
	_ domain.Embedder         = (*FakeEmbedder)(nil)
	_ domain.VectorStore      = (*FakeVectorStore)(nil)
	_ domain.NodeRepository   = (*FakeNodeRepository)(nil)
	_ domain.BundleRepository = (*FakeBundleRepository)(nil)
)

// ---------------------------------------------------------------------------
// FakeEmbedder
// ---------------------------------------------------------------------------

// FakeEmbedder is an in-memory implementation of domain.Embedder.
// Embed returns zero vectors of the configured dimensionality so tests can
// call it without an actual model.
type FakeEmbedder struct {
	dims int
}

// NewFakeEmbedder creates a FakeEmbedder with the given vector dimensionality.
func NewFakeEmbedder(dims int) *FakeEmbedder {
	return &FakeEmbedder{dims: dims}
}

// Embed returns a slice of zero-value float32 vectors, one per input text.
func (f *FakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, f.dims)
	}
	return out, nil
}

// Dims returns the embedding dimensionality.
func (f *FakeEmbedder) Dims() int { return f.dims }

// ---------------------------------------------------------------------------
// FakeVectorStore
// ---------------------------------------------------------------------------

// FakeVectorStore is an in-memory implementation of domain.VectorStore.
// All stored chunks are kept in a map keyed by EmbeddingChunk.ID.
type FakeVectorStore struct {
	mu     sync.RWMutex
	chunks map[string]domain.EmbeddingChunk
}

// NewFakeVectorStore creates an empty FakeVectorStore.
func NewFakeVectorStore() *FakeVectorStore {
	return &FakeVectorStore{chunks: make(map[string]domain.EmbeddingChunk)}
}

// Upsert inserts or replaces chunks by ID.
func (f *FakeVectorStore) Upsert(_ context.Context, chunks []domain.EmbeddingChunk) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range chunks {
		f.chunks[c.ID] = c
	}
	return nil
}

// Search returns up to topK chunks that match the given scope. The query
// vector is ignored — all matching chunks are returned with Score 1.0.
// Results are not guaranteed to be in a stable order.
func (f *FakeVectorStore) Search(_ context.Context, _ []float32, scope domain.Scope, topK int) ([]domain.ScoredChunk, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var results []domain.ScoredChunk
	for _, c := range f.chunks {
		if !chunkMatchesScope(c, scope) {
			continue
		}
		results = append(results, domain.ScoredChunk{
			Source:             c.BundleAlias + ":" + c.ConceptPath,
			ChunkIndex:         c.ChunkIndex,
			ChunkText:          c.Text,
			Score:              1.0,
			FrontmatterSummary: c.FrontmatterSummary,
		})
		if len(results) >= topK {
			break
		}
	}
	return results, nil
}

// Delete removes chunks by ID. Removing a non-existent ID is a no-op.
func (f *FakeVectorStore) Delete(_ context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		delete(f.chunks, id)
	}
	return nil
}

// Persist is a no-op for the fake.
func (f *FakeVectorStore) Persist(_ context.Context) error { return nil }

// Load is a no-op for the fake.
func (f *FakeVectorStore) Load(_ context.Context) error { return nil }

// chunkMatchesScope reports whether c falls within the given search scope.
func chunkMatchesScope(c domain.EmbeddingChunk, scope domain.Scope) bool {
	switch scope.Kind {
	case domain.ScopeGlobal:
		return true
	case domain.ScopeBundle:
		return c.BundleAlias == scope.BundleAlias
	case domain.ScopePath:
		return c.BundleAlias == scope.BundleAlias &&
			strings.HasPrefix(c.ConceptPath, scope.SubPath)
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// FakeNodeRepository
// ---------------------------------------------------------------------------

// FakeNodeRepository is an in-memory implementation of domain.NodeRepository.
// Concepts are keyed by ConceptRef.String() ("alias:relative/path.md").
// Reserved files are keyed by "alias:relPath".
type FakeNodeRepository struct {
	mu       sync.RWMutex
	concepts map[string]*domain.OKFConcept
	reserved map[string]string
}

// NewFakeNodeRepository creates an empty FakeNodeRepository.
func NewFakeNodeRepository() *FakeNodeRepository {
	return &FakeNodeRepository{
		concepts: make(map[string]*domain.OKFConcept),
		reserved: make(map[string]string),
	}
}

// Get retrieves a concept by ref. Returns domain.ErrNotFound if absent.
func (f *FakeNodeRepository) Get(_ context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.concepts[ref.String()]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

// Put stores or replaces a concept.
func (f *FakeNodeRepository) Put(_ context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.concepts[ref.String()] = concept
	return nil
}

// List returns refs for all stored concepts in bundleAlias whose RelativePath
// starts with subPath. If subPath is empty, all concepts in the bundle are
// returned.
func (f *FakeNodeRepository) List(_ context.Context, bundleAlias, subPath string) ([]domain.ConceptRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	bundlePrefix := bundleAlias + ":"
	pathPrefix := bundlePrefix
	if subPath != "" {
		pathPrefix = bundlePrefix + subPath
	}

	var refs []domain.ConceptRef
	for key := range f.concepts {
		if strings.HasPrefix(key, pathPrefix) {
			relPath := strings.TrimPrefix(key, bundlePrefix)
			refs = append(refs, domain.ConceptRef{
				BundleAlias:  bundleAlias,
				RelativePath: relPath,
			})
		}
	}
	return refs, nil
}

// ListTypes returns all distinct non-empty frontmatter type values across
// concepts in the given bundle.
func (f *FakeNodeRepository) ListTypes(_ context.Context, bundleAlias string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	prefix := bundleAlias + ":"
	seen := make(map[string]struct{})
	for key, concept := range f.concepts {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if t := concept.Frontmatter.Type; t != "" {
			seen[t] = struct{}{}
		}
	}

	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	return types, nil
}

// ReadReserved returns the content of a reserved file. Returns
// domain.ErrNotFound if the file has not been written.
func (f *FakeNodeRepository) ReadReserved(_ context.Context, bundleAlias, relPath string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	content, ok := f.reserved[bundleAlias+":"+relPath]
	if !ok {
		return "", domain.ErrNotFound
	}
	return content, nil
}

// WriteReserved stores or replaces a reserved file.
func (f *FakeNodeRepository) WriteReserved(_ context.Context, bundleAlias, relPath, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserved[bundleAlias+":"+relPath] = content
	return nil
}

// ---------------------------------------------------------------------------
// FakeBundleRepository
// ---------------------------------------------------------------------------

// FakeBundleRepository is an in-memory implementation of domain.BundleRepository.
// Bundles are keyed by Alias.
type FakeBundleRepository struct {
	mu      sync.RWMutex
	bundles map[string]domain.BundleEntry
}

// NewFakeBundleRepository creates an empty FakeBundleRepository.
func NewFakeBundleRepository() *FakeBundleRepository {
	return &FakeBundleRepository{bundles: make(map[string]domain.BundleEntry)}
}

// Get retrieves a bundle entry by alias. Returns domain.ErrNotFound if absent.
func (f *FakeBundleRepository) Get(_ context.Context, alias string) (*domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	entry, ok := f.bundles[alias]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &entry, nil
}

// Put stores or replaces a bundle entry.
func (f *FakeBundleRepository) Put(_ context.Context, entry domain.BundleEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[entry.Alias] = entry
	return nil
}

// Delete removes a bundle entry by alias. Removing a non-existent alias is a
// no-op.
func (f *FakeBundleRepository) Delete(_ context.Context, alias string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bundles, alias)
	return nil
}

// List returns all registered bundle entries. Order is not guaranteed.
func (f *FakeBundleRepository) List(_ context.Context) ([]domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	entries := make([]domain.BundleEntry, 0, len(f.bundles))
	for _, e := range f.bundles {
		entries = append(entries, e)
	}
	return entries, nil
}
