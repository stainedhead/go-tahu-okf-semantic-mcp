// Package domaintest provides in-memory fakes for domain interfaces for use in
// tests across the usecase and adapter layers. Import with the test build tag.
package domaintest

import (
	"context"
	"strings"
	"sync"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// BundleRepository is an in-memory fake implementing domain.BundleRepository.
type BundleRepository struct {
	mu      sync.RWMutex
	Bundles map[string]domain.BundleEntry
}

// NewBundleRepository returns an empty BundleRepository fake.
func NewBundleRepository() *BundleRepository {
	return &BundleRepository{Bundles: make(map[string]domain.BundleEntry)}
}

// Get implements domain.BundleRepository.
func (f *BundleRepository) Get(_ context.Context, alias string) (*domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.Bundles[alias]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &e, nil
}

// Put implements domain.BundleRepository.
func (f *BundleRepository) Put(_ context.Context, entry domain.BundleEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Bundles[entry.Alias] = entry
	return nil
}

// Delete implements domain.BundleRepository.
func (f *BundleRepository) Delete(_ context.Context, alias string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Bundles, alias)
	return nil
}

// List implements domain.BundleRepository.
func (f *BundleRepository) List(_ context.Context) ([]domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]domain.BundleEntry, 0, len(f.Bundles))
	for _, e := range f.Bundles {
		out = append(out, e)
	}
	return out, nil
}

// NodeRepository is an in-memory fake implementing domain.NodeRepository.
type NodeRepository struct {
	mu       sync.RWMutex
	Concepts map[string]*domain.OKFConcept
	Reserved map[string]string
}

// NewNodeRepository returns an empty NodeRepository fake.
func NewNodeRepository() *NodeRepository {
	return &NodeRepository{
		Concepts: make(map[string]*domain.OKFConcept),
		Reserved: make(map[string]string),
	}
}

// Get implements domain.NodeRepository.
func (f *NodeRepository) Get(_ context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.Concepts[ref.String()]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

// Put implements domain.NodeRepository.
func (f *NodeRepository) Put(_ context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Concepts[ref.String()] = concept
	return nil
}

// List implements domain.NodeRepository.
func (f *NodeRepository) List(_ context.Context, bundleAlias, subPath string) ([]domain.ConceptRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	bundlePrefix := bundleAlias + ":"
	pathPrefix := bundlePrefix
	if subPath != "" {
		pathPrefix = bundlePrefix + subPath
	}
	var refs []domain.ConceptRef
	for key := range f.Concepts {
		if strings.HasPrefix(key, pathPrefix) {
			refs = append(refs, domain.ConceptRef{
				BundleAlias:  bundleAlias,
				RelativePath: strings.TrimPrefix(key, bundlePrefix),
			})
		}
	}
	return refs, nil
}

// ListTypes implements domain.NodeRepository.
func (f *NodeRepository) ListTypes(_ context.Context, bundleAlias string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	prefix := bundleAlias + ":"
	seen := make(map[string]struct{})
	for key, c := range f.Concepts {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if t := c.Frontmatter.Type; t != "" {
			seen[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	return out, nil
}

// ReadReserved implements domain.NodeRepository.
func (f *NodeRepository) ReadReserved(_ context.Context, bundleAlias, relPath string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	content, ok := f.Reserved[bundleAlias+":"+relPath]
	if !ok {
		return "", domain.ErrNotFound
	}
	return content, nil
}

// WriteReserved implements domain.NodeRepository.
func (f *NodeRepository) WriteReserved(_ context.Context, bundleAlias, relPath, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Reserved[bundleAlias+":"+relPath] = content
	return nil
}

// Embedder is a simple fake domain.Embedder that returns fixed-size zero vectors.
type Embedder struct {
	Dims_ int
}

// Embed implements domain.Embedder.
func (e *Embedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, e.Dims_)
	}
	return out, nil
}

// Dims implements domain.Embedder.
func (e *Embedder) Dims() int { return e.Dims_ }

// VectorStore is an in-memory fake domain.VectorStore.
type VectorStore struct {
	mu     sync.RWMutex
	Chunks []domain.EmbeddingChunk
}

// Upsert implements domain.VectorStore.
func (v *VectorStore) Upsert(_ context.Context, chunks []domain.EmbeddingChunk) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Chunks = append(v.Chunks, chunks...)
	return nil
}

// DeleteByBundle implements domain.VectorStore.
func (v *VectorStore) DeleteByBundle(_ context.Context, alias string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	kept := v.Chunks[:0]
	for _, c := range v.Chunks {
		if c.BundleAlias != alias {
			kept = append(kept, c)
		}
	}
	v.Chunks = kept
	return nil
}

// DeleteByIDs implements domain.VectorStore.
func (v *VectorStore) DeleteByIDs(_ context.Context, ids []string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	kept := v.Chunks[:0]
	for _, c := range v.Chunks {
		if _, del := idSet[c.ID]; !del {
			kept = append(kept, c)
		}
	}
	v.Chunks = kept
	return nil
}

// Search implements domain.VectorStore.
func (v *VectorStore) Search(_ context.Context, _ []float32, _ domain.Scope, _ int) ([]domain.ScoredChunk, error) {
	return nil, nil
}

// Persist implements domain.VectorStore.
func (v *VectorStore) Persist(_ context.Context) error { return nil }

// Load implements domain.VectorStore.
func (v *VectorStore) Load(_ context.Context) error { return nil }
