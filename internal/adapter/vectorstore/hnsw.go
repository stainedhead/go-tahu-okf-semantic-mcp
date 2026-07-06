// Package vectorstore implements domain.VectorStore using an in-process HNSW
// graph provided by github.com/coder/hnsw.
package vectorstore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/coder/hnsw"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

const (
	// metaSuffix is appended to persistPath to form the chunk-metadata file.
	metaSuffix = ".meta"

	// oversampleFactor is the number of extra candidates fetched from the
	// HNSW graph when a non-global scope filter is applied. A higher value
	// improves recall at the cost of extra post-filter work.
	oversampleFactor = 20
)

// HNSWStore implements domain.VectorStore using an in-process HNSW graph.
// Vectors are stored in the graph (keyed by EmbeddingChunk.ID); all other
// chunk metadata is stored in a parallel map. Both are serialised to disk by
// Persist and restored by Load.
//
// HNSWStore is thread-safe: read operations hold the read lock; write
// operations hold the exclusive lock.
type HNSWStore struct {
	mu          sync.RWMutex
	graph       *hnsw.Graph[string]
	chunks      map[string]domain.EmbeddingChunk
	persistPath string
	dims        int
}

// Compile-time conformance check.
var _ domain.VectorStore = (*HNSWStore)(nil)

// New creates an empty HNSWStore. Call Load to restore a previously persisted
// index.
//
//   - persistPath: file path written by Persist and read by Load.
//     A companion file at persistPath+".meta" holds chunk metadata.
//   - dims: expected embedding dimensionality; used for early validation in
//     Upsert.
//   - efConstruction: HNSW EfSearch parameter — controls graph quality during
//     both construction and search (higher = better recall, more memory).
//   - m: HNSW M parameter — maximum neighbours per node.
func New(persistPath string, dims int, efConstruction int, m int) (*HNSWStore, error) {
	if dims <= 0 {
		return nil, fmt.Errorf("vectorstore.New: dims must be > 0, got %d", dims)
	}
	if efConstruction <= 0 {
		return nil, fmt.Errorf("vectorstore.New: efConstruction must be > 0, got %d", efConstruction)
	}
	if m <= 0 {
		return nil, fmt.Errorf("vectorstore.New: m must be > 0, got %d", m)
	}

	g := hnsw.NewGraph[string]()
	g.M = m
	g.EfSearch = efConstruction
	g.Distance = hnsw.CosineDistance

	return &HNSWStore{
		graph:       g,
		chunks:      make(map[string]domain.EmbeddingChunk),
		persistPath: persistPath,
		dims:        dims,
	}, nil
}

// Upsert inserts or replaces the given chunks in the HNSW graph and the
// metadata map. Replacing an existing chunk re-inserts it at a new position in
// the graph (the library handles deletion of the old entry internally).
func (s *HNSWStore) Upsert(_ context.Context, chunks []domain.EmbeddingChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range chunks {
		if len(c.Embedding) == 0 {
			return fmt.Errorf("vectorstore.Upsert: chunk %q has empty embedding", c.ID)
		}
		if len(c.Embedding) != s.dims {
			return fmt.Errorf(
				"vectorstore.Upsert: chunk %q has %d dims, want %d",
				c.ID, len(c.Embedding), s.dims,
			)
		}
		if isZeroNorm(c.Embedding) {
			continue // silently skip; BM25 OOV produces zero vectors
		}
		s.graph.Add(hnsw.MakeNode(c.ID, c.Embedding))
		s.chunks[c.ID] = c
	}
	return nil
}

// Search returns up to topK chunks nearest to query, filtered by scope.
// Scores are cosine similarities in [−1, 1] (typically [0, 1] for normalised
// embeddings). Results are ordered by score descending.
//
// For non-global scopes the graph is over-fetched by oversampleFactor to
// mitigate the precision loss that post-filter pruning causes.
func (s *HNSWStore) Search(_ context.Context, query []float32, scope domain.Scope, topK int) ([]domain.ScoredChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if topK <= 0 {
		return nil, nil
	}
	if s.graph.Len() == 0 {
		return nil, nil
	}

	fetchK := topK
	if scope.Kind != domain.ScopeGlobal {
		fetchK = topK * oversampleFactor
	}

	nodes := s.graph.Search(query, fetchK)

	type candidate struct {
		chunk domain.EmbeddingChunk
		score float32
	}

	candidates := make([]candidate, 0, len(nodes))
	for _, n := range nodes {
		c, ok := s.chunks[n.Key]
		if !ok {
			continue
		}
		if !chunkMatchesScope(c, scope) {
			continue
		}
		score := float32(1) - hnsw.CosineDistance(query, n.Value)
		if math.IsNaN(float64(score)) {
			continue
		}
		candidates = append(candidates, candidate{chunk: c, score: score})
	}

	// Stable sort: higher score first. Stable preserves insertion order for
	// equal-score results, giving deterministic output.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	out := make([]domain.ScoredChunk, 0, len(candidates))
	for _, cd := range candidates {
		out = append(out, domain.ScoredChunk{
			Source:             cd.chunk.BundleAlias + ":" + cd.chunk.ConceptPath,
			ChunkIndex:         cd.chunk.ChunkIndex,
			ChunkText:          cd.chunk.Text,
			Score:              cd.score,
			FrontmatterSummary: cd.chunk.FrontmatterSummary,
		})
	}
	return out, nil
}

// Delete removes chunks by ID from both the HNSW graph and the metadata map.
// IDs not present in the store are silently ignored.
func (s *HNSWStore) Delete(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		s.graph.Delete(id)
		delete(s.chunks, id)
	}
	return nil
}

// Persist writes the HNSW graph and chunk metadata to durable storage
// atomically (write-to-temp + rename). Two files are written:
//
//   - persistPath      – binary HNSW graph (coder/hnsw binary encoding)
//   - persistPath+".meta" – JSON-encoded chunk metadata map
func (s *HNSWStore) Persist(_ context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.writeGraph(); err != nil {
		return fmt.Errorf("vectorstore.Persist: write graph: %w", err)
	}
	if err := s.writeMeta(); err != nil {
		return fmt.Errorf("vectorstore.Persist: write meta: %w", err)
	}
	return nil
}

// Load restores the HNSW graph and chunk metadata from durable storage.
// If the index file does not exist this is a no-op (cold start).
func (s *HNSWStore) Load(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := os.Stat(s.persistPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil // cold start — nothing to load
	}
	if err != nil {
		return fmt.Errorf("vectorstore.Load: stat %q: %w", s.persistPath, err)
	}

	if err := s.readGraph(); err != nil {
		return fmt.Errorf("vectorstore.Load: read graph: %w", err)
	}
	if err := s.readMeta(); err != nil {
		return fmt.Errorf("vectorstore.Load: read meta: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// writeGraph serialises the HNSW graph to persistPath via a temp file.
func (s *HNSWStore) writeGraph() error {
	dir := filepath.Dir(s.persistPath)
	tmp, err := os.CreateTemp(dir, ".hnsw-graph-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	// Track whether the rename succeeded so the deferred cleanup is correct.
	var renamed bool
	defer func() {
		_ = tmp.Close()
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	bw := bufio.NewWriter(tmp)
	if err := s.graph.Export(bw); err != nil {
		return fmt.Errorf("export graph: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flush graph: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, s.persistPath); err != nil {
		return fmt.Errorf("rename graph: %w", err)
	}
	renamed = true
	return nil
}

// writeMeta serialises the chunk metadata map to persistPath+metaSuffix.
func (s *HNSWStore) writeMeta() error {
	metaPath := s.persistPath + metaSuffix
	dir := filepath.Dir(metaPath)
	tmp, err := os.CreateTemp(dir, ".hnsw-meta-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	var renamed bool
	defer func() {
		_ = tmp.Close()
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	enc := json.NewEncoder(tmp)
	if err := enc.Encode(s.chunks); err != nil {
		return fmt.Errorf("encode meta: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, metaPath); err != nil {
		return fmt.Errorf("rename meta: %w", err)
	}
	renamed = true
	return nil
}

// readGraph deserialises the HNSW graph from persistPath.
func (s *HNSWStore) readGraph() error {
	f, err := os.Open(s.persistPath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := s.graph.Import(bufio.NewReader(f)); err != nil {
		return fmt.Errorf("import graph: %w", err)
	}
	return nil
}

// readMeta deserialises the chunk metadata map from persistPath+metaSuffix.
// If the file does not exist (e.g. the graph was empty on last save) this is
// a no-op.
func (s *HNSWStore) readMeta() error {
	metaPath := s.persistPath + metaSuffix
	f, err := os.Open(metaPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := json.NewDecoder(f).Decode(&s.chunks); err != nil {
		return fmt.Errorf("decode meta: %w", err)
	}
	return nil
}

// isZeroNorm reports whether all elements of v are zero.  A zero-norm vector
// produces NaN from cosine-distance computations (division by zero magnitude)
// and must be excluded from the HNSW graph.
func isZeroNorm(v []float32) bool {
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}

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
