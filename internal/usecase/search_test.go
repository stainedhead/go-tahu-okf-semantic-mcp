package usecase_test

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
)

// ---------------------------------------------------------------------------
// Compile-time interface conformance checks
// ---------------------------------------------------------------------------

var (
	_ domain.Embedder    = (*fakeEmbedder)(nil)
	_ domain.VectorStore = (*fakeVectorStore)(nil)
)

// ---------------------------------------------------------------------------
// fakeEmbedder
// ---------------------------------------------------------------------------

// fakeEmbedder is a deterministic in-memory Embedder for tests.
// It returns a vector of fixed dimensionality where element 0 encodes the
// input text via a simple polynomial hash, giving a unique vector per
// distinct text without any external dependency.
type fakeEmbedder struct {
	dims int
}

func newFakeEmbedder(dims int) *fakeEmbedder {
	return &fakeEmbedder{dims: dims}
}

// Embed returns one deterministic []float32 per input text.
func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, f.dims)
		// Polynomial hash over runes — deterministic, unique per text.
		var h uint32
		for _, r := range t {
			h = h*31 + uint32(r) //nolint:gosec // G115: test hash, not security-sensitive
		}
		v[0] = float32(h%1000) / 1000.0
		out[i] = v
	}
	return out, nil
}

func (f *fakeEmbedder) Dims() int { return f.dims }

// ---------------------------------------------------------------------------
// fakeVectorStore
// ---------------------------------------------------------------------------

// fakeVectorStore is an in-memory VectorStore for tests.
// Search ignores the query vector and returns matching chunks sorted by
// ChunkIndex ascending (i.e. descending score), with Score = 1/(ChunkIndex+1),
// simulating a real ranked retrieval result.
type fakeVectorStore struct {
	mu     sync.RWMutex
	chunks map[string]domain.EmbeddingChunk
}

func newFakeVectorStore() *fakeVectorStore {
	return &fakeVectorStore{chunks: make(map[string]domain.EmbeddingChunk)}
}

func (f *fakeVectorStore) Upsert(_ context.Context, chunks []domain.EmbeddingChunk) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range chunks {
		f.chunks[c.ID] = c
	}
	return nil
}

// Search filters by scope, sorts by ChunkIndex ascending, truncates to topK,
// and assigns Score = 1/(ChunkIndex+1) so that lower indices have higher scores.
func (f *fakeVectorStore) Search(_ context.Context, _ []float32, scope domain.Scope, topK int) ([]domain.ScoredChunk, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var matched []domain.EmbeddingChunk
	for _, c := range f.chunks {
		if fakeMatchesScope(c, scope) {
			matched = append(matched, c)
		}
	}
	// Sort ascending by ChunkIndex → descending score.
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ChunkIndex < matched[j].ChunkIndex
	})
	if topK > 0 && len(matched) > topK {
		matched = matched[:topK]
	}

	out := make([]domain.ScoredChunk, len(matched))
	for i, c := range matched {
		out[i] = domain.ScoredChunk{
			Source:             c.BundleAlias + ":" + c.ConceptPath,
			ChunkIndex:         c.ChunkIndex,
			ChunkText:          c.Text,
			Score:              1.0 / float32(c.ChunkIndex+1),
			FrontmatterSummary: c.FrontmatterSummary,
		}
	}
	return out, nil
}

func (f *fakeVectorStore) Delete(_ context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		delete(f.chunks, id)
	}
	return nil
}

func (f *fakeVectorStore) Persist(_ context.Context) error { return nil }
func (f *fakeVectorStore) Load(_ context.Context) error    { return nil }

// chunkCount returns the number of chunks currently held in the store.
func (f *fakeVectorStore) chunkCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.chunks)
}

// fakeMatchesScope reports whether c falls within the given search scope.
func fakeMatchesScope(c domain.EmbeddingChunk, scope domain.Scope) bool {
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
// helpers
// ---------------------------------------------------------------------------

// makeChunk builds an EmbeddingChunk with the canonical ID format.
func makeChunk(alias, path string, idx int, text string) domain.EmbeddingChunk {
	return domain.EmbeddingChunk{
		ID:          alias + ":" + path + ":" + strconv.Itoa(idx),
		BundleAlias: alias,
		ConceptPath: path,
		ChunkIndex:  idx,
		Text:        text,
	}
}

// upsertChunks is a test helper that fails immediately on Upsert error.
func upsertChunks(t *testing.T, vs *fakeVectorStore, chunks ...domain.EmbeddingChunk) {
	t.Helper()
	if err := vs.Upsert(context.Background(), chunks); err != nil {
		t.Fatalf("upsertChunks: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSearchService_SemanticSearch_FR012 verifies that SemanticSearch returns
// chunks in descending score order (ascending ChunkIndex via fake store).
// Spec reference: FR-012, G3.
func TestSearchService_SemanticSearch_FR012(t *testing.T) {
	vs := newFakeVectorStore()
	upsertChunks(t, vs,
		makeChunk("kb", "concepts/alpha.md", 0, "first chunk"),
		makeChunk("kb", "concepts/alpha.md", 1, "second chunk"),
		makeChunk("kb", "concepts/alpha.md", 2, "third chunk"),
	)

	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: vs,
	}

	got, err := svc.SemanticSearch(context.Background(), "alpha", domain.Scope{Kind: domain.ScopeGlobal}, 3)
	if err != nil {
		t.Fatalf("SemanticSearch returned unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}

	// Assert descending score order: indices 0 → 1 → 2, scores 1.0 → 0.5 → 0.333.
	wantIndices := []int{0, 1, 2}
	for i, sc := range got {
		if sc.ChunkIndex != wantIndices[i] {
			t.Errorf("result[%d]: want ChunkIndex=%d, got %d", i, wantIndices[i], sc.ChunkIndex)
		}
		wantScore := 1.0 / float32(wantIndices[i]+1)
		if sc.Score != wantScore {
			t.Errorf("result[%d]: want Score=%v, got %v", i, wantScore, sc.Score)
		}
	}
}

// TestSearchService_KeywordSearch_FR013 verifies that KeywordSearch returns the
// same shape and order as SemanticSearch, confirming the interface is identical
// regardless of the underlying Embedder. Spec reference: FR-013, G3.
func TestSearchService_KeywordSearch_FR013(t *testing.T) {
	vs := newFakeVectorStore()
	upsertChunks(t, vs,
		makeChunk("kb", "concepts/beta.md", 0, "top result"),
		makeChunk("kb", "concepts/beta.md", 1, "second result"),
	)

	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: vs,
	}

	got, err := svc.KeywordSearch(context.Background(), "beta", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err != nil {
		t.Fatalf("KeywordSearch returned unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
	// Assert order: index 0 before index 1.
	if got[0].ChunkIndex != 0 || got[1].ChunkIndex != 1 {
		t.Errorf("expected ChunkIndex order [0,1], got [%d,%d]",
			got[0].ChunkIndex, got[1].ChunkIndex)
	}
	// Assert scores match 1/(index+1).
	if got[0].Score != 1.0 {
		t.Errorf("result[0]: want Score=1.0, got %v", got[0].Score)
	}
	if got[1].Score != 0.5 {
		t.Errorf("result[1]: want Score=0.5, got %v", got[1].Score)
	}
}

// TestSearchService_RAGSearch_FiltersMinScore_FR014 verifies that RAGSearch
// omits chunks whose Score is below minScore, returning only those that meet
// the threshold. Spec reference: FR-014, G8.
func TestSearchService_RAGSearch_FiltersMinScore_FR014(t *testing.T) {
	vs := newFakeVectorStore()
	// ChunkIndex 0 → Score 1.0 (passes minScore=0.4).
	// ChunkIndex 2 → Score 0.333 (below minScore=0.4, must be excluded).
	upsertChunks(t, vs,
		makeChunk("kb", "concepts/gamma.md", 0, "high-score chunk"),
		makeChunk("kb", "concepts/gamma.md", 2, "low-score chunk"),
	)

	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: vs,
	}

	got, err := svc.RAGSearch(context.Background(), "gamma", domain.Scope{Kind: domain.ScopeGlobal}, 5, 0.4)
	if err != nil {
		t.Fatalf("RAGSearch returned unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk above threshold, got %d", len(got))
	}
	if got[0].ChunkIndex != 0 {
		t.Errorf("expected ChunkIndex=0 (score 1.0), got ChunkIndex=%d", got[0].ChunkIndex)
	}
}

// TestSearchService_RAGSearch_EmptyResult_FR014 verifies that RAGSearch returns
// an empty (non-nil) slice and no error when all retrieved chunks fall below
// minScore. Spec reference: FR-014 "Returns empty list if no chunks meet
// threshold", G8.
func TestSearchService_RAGSearch_EmptyResult_FR014(t *testing.T) {
	vs := newFakeVectorStore()
	// ChunkIndex 4 → Score 0.2, which is below minScore=0.5.
	upsertChunks(t, vs,
		makeChunk("kb", "concepts/delta.md", 4, "below-threshold chunk"),
	)

	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: vs,
	}

	got, err := svc.RAGSearch(context.Background(), "delta", domain.Scope{Kind: domain.ScopeGlobal}, 5, 0.5)
	if err != nil {
		t.Fatalf("RAGSearch must not error when no chunks meet threshold: %v", err)
	}
	if got == nil {
		t.Fatal("RAGSearch must return a non-nil slice, not nil, when result is empty")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d chunks", len(got))
	}
}

// TestSearchService_ScopeBundle_FR012 verifies that a ScopeBundle search
// returns only chunks belonging to the specified bundle, excluding chunks from
// all other bundles. Spec reference: FR-012 "Respects scope (global / bundle /
// path)", G3.
func TestSearchService_ScopeBundle_FR012(t *testing.T) {
	vs := newFakeVectorStore()
	upsertChunks(t, vs,
		makeChunk("alpha", "runbooks/deploy.md", 0, "alpha bundle chunk"),
		makeChunk("beta", "guides/onboard.md", 0, "beta bundle chunk"),
		makeChunk("alpha", "runbooks/rollback.md", 1, "another alpha chunk"),
	)

	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: vs,
	}

	scope := domain.Scope{Kind: domain.ScopeBundle, BundleAlias: "alpha"}
	got, err := svc.SemanticSearch(context.Background(), "deploy", scope, 10)
	if err != nil {
		t.Fatalf("SemanticSearch returned unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks from bundle 'alpha', got %d", len(got))
	}
	for _, sc := range got {
		if !strings.HasPrefix(sc.Source, "alpha:") {
			t.Errorf("expected all results from bundle 'alpha', got source %q", sc.Source)
		}
	}
}
