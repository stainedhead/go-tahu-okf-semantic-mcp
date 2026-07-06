package vectorstore_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/vectorstore"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	testDims           = 4
	testEfConstruction = 20
	testM              = 4
)

// newStore creates an HNSWStore with a temp directory as the persist path,
// suitable for use in a single test. The caller does not need to clean up —
// t.TempDir is used.
func newStore(t *testing.T) *vectorstore.HNSWStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.hnsw")
	s, err := vectorstore.New(path, testDims, testEfConstruction, testM)
	if err != nil {
		t.Fatalf("vectorstore.New: %v", err)
	}
	return s
}

// makeChunk builds an EmbeddingChunk with the given fields and a normalised
// embedding. The embedding points in the direction of the unit vector with 1.0
// at position vecIdx and the remaining components filled with small values as
// specified by extra so that cosine similarities are clearly ordered.
func makeChunk(id, alias, path string, chunkIdx int, embedding []float32) domain.EmbeddingChunk {
	return domain.EmbeddingChunk{
		ID:                 id,
		BundleAlias:        alias,
		ConceptPath:        path,
		ChunkIndex:         chunkIdx,
		Text:               "text for " + id,
		Embedding:          embedding,
		FrontmatterSummary: "concept:" + id,
	}
}

// norm returns the L2-normalised form of v (in-place).
func norm(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	mag := float32(math.Sqrt(sum))
	if mag == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / mag
	}
	return out
}

// approxEqual returns true if |a−b| < eps.
func approxEqual(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

// ---------------------------------------------------------------------------
// TestHNSWStore_UpsertAndSearch_ReturnsRankedResults
//
// Spec: Search returns up to topK results ordered by cosine-similarity
// descending (closest first) — SpecSearch1.
// ---------------------------------------------------------------------------

func TestHNSWStore_UpsertAndSearch_ReturnsRankedResults(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Three vectors with clearly separated cosine similarities when compared
	// to the query [1, 0, 0, 0]:
	//   A: [1, 0, 0, 0]      → similarity ≈ 1.0  (perfect match)
	//   B: norm([3, 4, 0, 0]) → similarity = 0.6
	//   C: [0, 1, 0, 0]      → similarity = 0.0
	query := []float32{1, 0, 0, 0}
	chunkA := makeChunk("a:docs/a:0", "docs", "a.md", 0, []float32{1, 0, 0, 0})
	chunkB := makeChunk("a:docs/b:0", "docs", "b.md", 0, norm([]float32{3, 4, 0, 0}))
	chunkC := makeChunk("a:docs/c:0", "docs", "c.md", 0, []float32{0, 1, 0, 0})

	if err := s.Upsert(ctx, []domain.EmbeddingChunk{chunkA, chunkB, chunkC}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	scope := domain.Scope{Kind: domain.ScopeGlobal}
	results, err := s.Search(ctx, query, scope, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Search returned no results")
	}

	// Top result must be chunk A (similarity ≈ 1.0).
	if results[0].Source != "docs:a.md" {
		t.Errorf("top result source = %q, want %q", results[0].Source, "docs:a.md")
	}
	if !approxEqual(results[0].Score, 1.0, 1e-5) {
		t.Errorf("top result score = %f, want ≈ 1.0", results[0].Score)
	}

	// Results must be ordered descending by score.
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results[%d].Score=%f > results[%d].Score=%f — not descending",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

// ---------------------------------------------------------------------------
// TestHNSWStore_Search_ScopeBundleFilters
//
// Spec: ScopeBundle restricts results to the given bundle alias — SpecSearch2.
// ---------------------------------------------------------------------------

func TestHNSWStore_Search_ScopeBundleFilters(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	query := []float32{1, 0, 0, 0}

	// Two chunks in "alpha", one in "beta". The alpha chunks are farther from
	// the query than the beta chunk so that without scope filtering the beta
	// chunk would rank first.
	chunkAlpha1 := makeChunk("alpha:p1:0", "alpha", "p1.md", 0, norm([]float32{3, 4, 0, 0})) // sim=0.6
	chunkAlpha2 := makeChunk("alpha:p2:0", "alpha", "p2.md", 0, norm([]float32{1, 1, 0, 0})) // sim≈0.71
	chunkBeta := makeChunk("beta:p1:0", "beta", "p1.md", 0, []float32{1, 0, 0, 0})           // sim=1.0

	if err := s.Upsert(ctx, []domain.EmbeddingChunk{chunkAlpha1, chunkAlpha2, chunkBeta}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	scope := domain.Scope{Kind: domain.ScopeBundle, BundleAlias: "alpha"}
	results, err := s.Search(ctx, query, scope, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Search returned no results for bundle 'alpha'")
	}

	for _, r := range results {
		// All returned sources must start with "alpha:".
		if len(r.Source) < 6 || r.Source[:6] != "alpha:" {
			t.Errorf("result source %q is not in bundle 'alpha'", r.Source)
		}
	}

	// The beta chunk must not appear.
	for _, r := range results {
		if r.Source == "beta:p1.md" {
			t.Error("result from bundle 'beta' leaked into ScopeBundle='alpha' search")
		}
	}
}

// ---------------------------------------------------------------------------
// TestHNSWStore_Search_ScopePathFilters
//
// Spec: ScopePath restricts results to the given sub-path prefix within a
// bundle — SpecSearch3.
// ---------------------------------------------------------------------------

func TestHNSWStore_Search_ScopePathFilters(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	query := []float32{1, 0, 0, 0}

	// Chunks in "kb" bundle at different sub-paths.
	inScope := makeChunk("kb:notes/idea:0", "kb", "notes/idea.md", 0, []float32{1, 0, 0, 0})
	outScope := makeChunk("kb:archive/old:0", "kb", "archive/old.md", 0, []float32{1, 0, 0, 0})
	otherBundle := makeChunk("other:notes/idea:0", "other", "notes/idea.md", 0, []float32{1, 0, 0, 0})

	if err := s.Upsert(ctx, []domain.EmbeddingChunk{inScope, outScope, otherBundle}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	scope := domain.Scope{
		Kind:        domain.ScopePath,
		BundleAlias: "kb",
		SubPath:     "notes/",
	}
	results, err := s.Search(ctx, query, scope, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Search returned no results for path scope 'kb:notes/'")
	}

	for _, r := range results {
		if r.Source != "kb:notes/idea.md" {
			t.Errorf("unexpected result source %q — want only 'kb:notes/idea.md'", r.Source)
		}
	}
}

// ---------------------------------------------------------------------------
// TestHNSWStore_PersistAndLoad_SameResults
//
// Spec: After Persist+Load the store returns the same ranked results — SpecPersist1.
// ---------------------------------------------------------------------------

func TestHNSWStore_PersistAndLoad_SameResults(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	idxPath := filepath.Join(dir, "index.hnsw")

	// --- build and persist ---
	s1, err := vectorstore.New(idxPath, testDims, testEfConstruction, testM)
	if err != nil {
		t.Fatalf("New (s1): %v", err)
	}

	query := []float32{1, 0, 0, 0}
	chunks := []domain.EmbeddingChunk{
		makeChunk("kb:a:0", "kb", "a.md", 0, []float32{1, 0, 0, 0}),
		makeChunk("kb:b:0", "kb", "b.md", 0, norm([]float32{3, 4, 0, 0})),
		makeChunk("kb:c:0", "kb", "c.md", 0, []float32{0, 1, 0, 0}),
	}
	if err := s1.Upsert(ctx, chunks); err != nil {
		t.Fatalf("Upsert (s1): %v", err)
	}

	scope := domain.Scope{Kind: domain.ScopeGlobal}
	before, err := s1.Search(ctx, query, scope, 3)
	if err != nil {
		t.Fatalf("Search (s1 before): %v", err)
	}
	if err := s1.Persist(ctx); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// Verify the index file was created.
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("index file missing after Persist: %v", err)
	}
	if _, err := os.Stat(idxPath + ".meta"); err != nil {
		t.Fatalf("meta file missing after Persist: %v", err)
	}

	// --- restore into a fresh store and compare ---
	s2, err := vectorstore.New(idxPath, testDims, testEfConstruction, testM)
	if err != nil {
		t.Fatalf("New (s2): %v", err)
	}
	if err := s2.Load(ctx); err != nil {
		t.Fatalf("Load (s2): %v", err)
	}

	after, err := s2.Search(ctx, query, scope, 3)
	if err != nil {
		t.Fatalf("Search (s2 after): %v", err)
	}

	if len(before) != len(after) {
		t.Fatalf("result count mismatch: before=%d after=%d", len(before), len(after))
	}

	const eps = float32(1e-4)
	for i := range before {
		if before[i].Source != after[i].Source {
			t.Errorf("result[%d] source: before=%q after=%q", i, before[i].Source, after[i].Source)
		}
		if !approxEqual(before[i].Score, after[i].Score, eps) {
			t.Errorf("result[%d] score: before=%f after=%f (eps=%f)",
				i, before[i].Score, after[i].Score, eps)
		}
		if before[i].ChunkText != after[i].ChunkText {
			t.Errorf("result[%d] ChunkText: before=%q after=%q",
				i, before[i].ChunkText, after[i].ChunkText)
		}
	}
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// TestHNSWStore_ZeroVector_NoNaNScores
//
// Spec: Upsert silently skips zero-norm vectors; Search never produces NaN
// scores — SpecFix1.
// ---------------------------------------------------------------------------

func TestHNSWStore_ZeroVector_NoNaNScores(t *testing.T) {
	s, err := vectorstore.New(t.TempDir()+"/idx", 4, 200, 16)
	require.NoError(t, err)
	zero := make([]float32, 4)
	real := []float32{1, 0, 0, 0}
	_ = s.Upsert(context.Background(), []domain.EmbeddingChunk{
		{ID: "z", BundleAlias: "b", ConceptPath: "z.md", Text: "zero", Embedding: zero},
		{ID: "a", BundleAlias: "b", ConceptPath: "a.md", Text: "a", Embedding: real},
	})
	results, err := s.Search(context.Background(), real, domain.Scope{Kind: domain.ScopeGlobal}, 5)
	require.NoError(t, err)
	for _, r := range results {
		if math.IsNaN(float64(r.Score)) {
			t.Errorf("got NaN score for source %s", r.Source)
		}
	}
}

// TestHNSWStore_AllOOV_Search_ReturnsEmpty
//
// Spec: A zero-vector query (all-OOV) returns an empty result set — SpecFix2.
// ---------------------------------------------------------------------------

func TestHNSWStore_AllOOV_Search_ReturnsEmpty(t *testing.T) {
	s, err := vectorstore.New(t.TempDir()+"/idx", 4, 200, 16)
	require.NoError(t, err)
	zero := make([]float32, 4)
	_ = s.Upsert(context.Background(), []domain.EmbeddingChunk{
		{ID: "z", BundleAlias: "b", ConceptPath: "z.md", Text: "zero", Embedding: zero},
	})
	results, err := s.Search(context.Background(), zero, domain.Scope{Kind: domain.ScopeGlobal}, 5)
	require.NoError(t, err)
	if len(results) != 0 {
		t.Errorf("expected empty results for zero-vector query, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// TestHNSWStore_Load_ResetsExistingState
//
// Spec: Load completely replaces in-memory state from disk; it does not merge
// with existing chunks — SpecFix3-Reset.
// ---------------------------------------------------------------------------

func TestHNSWStore_Load_ResetsExistingState(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/idx"

	// Build s1 with chunk "old", persist to disk.
	s1, err := vectorstore.New(path, testDims, testEfConstruction, testM)
	require.NoError(t, err)
	require.NoError(t, s1.Upsert(context.Background(), []domain.EmbeddingChunk{
		{ID: "old", BundleAlias: "b", ConceptPath: "old.md", Embedding: []float32{1, 0, 0, 0}},
	}))
	require.NoError(t, s1.Persist(context.Background()))

	// Build s2 from the same path, add "other" without persisting.
	s2, err := vectorstore.New(path, testDims, testEfConstruction, testM)
	require.NoError(t, err)
	require.NoError(t, s2.Upsert(context.Background(), []domain.EmbeddingChunk{
		{ID: "other", BundleAlias: "b", ConceptPath: "other.md", Embedding: []float32{0, 1, 0, 0}},
	}))

	// Load should reset s2 to only contain what's on disk ("old").
	require.NoError(t, s2.Load(context.Background()))

	results, err := s2.Search(context.Background(), []float32{0, 1, 0, 0}, domain.Scope{Kind: domain.ScopeGlobal}, 5)
	require.NoError(t, err)
	for _, r := range results {
		if r.Source == "b:other.md" {
			t.Error("Load should have reset state; 'other' should not be present after Load")
		}
	}
}

// TestHNSWStore_Load_ValidatesDims
//
// Spec: Load returns an error when the persisted embedding dimensionality
// does not match the configured dims — SpecFix3-Dims.
// ---------------------------------------------------------------------------

func TestHNSWStore_Load_ValidatesDims(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/idx"

	// Persist a store with dims=4.
	s1, err := vectorstore.New(path, testDims, testEfConstruction, testM)
	require.NoError(t, err)
	require.NoError(t, s1.Upsert(context.Background(), []domain.EmbeddingChunk{
		{ID: "a", BundleAlias: "b", ConceptPath: "a.md", Embedding: []float32{1, 0, 0, 0}},
	}))
	require.NoError(t, s1.Persist(context.Background()))

	// Load into a store configured with dims=8 — must return an error.
	s2, err := vectorstore.New(path, 8, testEfConstruction, testM)
	require.NoError(t, err)
	if err := s2.Load(context.Background()); err == nil {
		t.Error("Load should return an error when persisted dims differ from configured dims")
	}
}

// ---------------------------------------------------------------------------
// TestHNSWStore_Load_NoopWhenFileAbsent
//
// Spec: Load is a no-op when the index file does not exist (cold start) — AC-G7.
// ---------------------------------------------------------------------------

func TestHNSWStore_Load_NoopWhenFileAbsent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	idxPath := filepath.Join(dir, "nonexistent.hnsw")

	s, err := vectorstore.New(idxPath, testDims, testEfConstruction, testM)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Load must succeed even though the file does not exist.
	if err := s.Load(ctx); err != nil {
		t.Fatalf("Load returned error on cold start: %v", err)
	}

	// The store must be empty after a cold-start Load.
	scope := domain.Scope{Kind: domain.ScopeGlobal}
	results, err := s.Search(ctx, []float32{1, 0, 0, 0}, scope, 5)
	if err != nil {
		t.Fatalf("Search after cold-start Load: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results after cold-start Load, got %d", len(results))
	}
}
