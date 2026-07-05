// Package usecase_test — error-path coverage tests (FIX-007).
//
// These tests target the uncovered branches in search.go, concept.go, and
// bundle.go that keep the package below the ≥90% coverage gate.
//
// Each error-injecting fake embeds the working in-memory fake from the other
// *_test.go files in this package, overriding only the single method under
// test, so the remaining methods still behave correctly.
package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
)

// ---------------------------------------------------------------------------
// Error-injecting fakes — Embedder / VectorStore (for search.go)
// ---------------------------------------------------------------------------

// errEmbedder always returns an error from Embed.
type errEmbedder struct{}

func (e *errEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, errors.New("embed failed")
}
func (e *errEmbedder) Dims() int { return 0 }

// emptyVecsEmbedder returns an empty (length-zero) slice from Embed to
// exercise the "embedder returned no vectors" guard.
type emptyVecsEmbedder struct{}

func (e *emptyVecsEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return [][]float32{}, nil
}
func (e *emptyVecsEmbedder) Dims() int { return 0 }

// errSearchVS overrides Search to return an error; all other methods
// delegate to the embedded working store.
type errSearchVS struct{ *fakeVectorStore }

func (e *errSearchVS) Search(_ context.Context, _ []float32, _ domain.Scope, _ int) ([]domain.ScoredChunk, error) {
	return nil, errors.New("search failed")
}

// errUpsertVS overrides Upsert to return an error.
type errUpsertVS struct{ *fakeVectorStore }

func (e *errUpsertVS) Upsert(_ context.Context, _ []domain.EmbeddingChunk) error {
	return errors.New("upsert failed")
}

// ---------------------------------------------------------------------------
// Error-injecting fakes — NodeRepository (for concept.go / bundle.go)
// ---------------------------------------------------------------------------

// errListNodeRepo overrides List to return a non-ErrNotFound error, so that
// callers that distinguish the two cases reach their fallback error path.
type errListNodeRepo struct{ *fakeNodeRepo }

func (r *errListNodeRepo) List(_ context.Context, _ string, _ string) ([]domain.ConceptRef, error) {
	return nil, errors.New("list failed")
}

// errListTypesNodeRepo overrides ListTypes to return an error.
type errListTypesNodeRepo struct{ *fakeNodeRepo }

func (r *errListTypesNodeRepo) ListTypes(_ context.Context, _ string) ([]string, error) {
	return nil, errors.New("list types failed")
}

// errPutNodeRepo overrides Put to return an error.
type errPutNodeRepo struct{ *fakeNodeRepo }

func (r *errPutNodeRepo) Put(_ context.Context, _ domain.ConceptRef, _ *domain.OKFConcept) error {
	return errors.New("put failed")
}

// errReadReservedNodeRepo overrides ReadReserved to return a non-ErrNotFound
// error, triggering the early-return path in appendLog.
type errReadReservedNodeRepo struct{ *fakeNodeRepo }

func (r *errReadReservedNodeRepo) ReadReserved(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("read reserved failed")
}

// errGetNodeRepo overrides Get to return an error, simulating a concept fetch
// failure during re-indexing.
type errGetNodeRepo struct{ *fakeNodeRepo }

func (r *errGetNodeRepo) Get(_ context.Context, _ domain.ConceptRef) (*domain.OKFConcept, error) {
	return nil, errors.New("get failed")
}

// ---------------------------------------------------------------------------
// Error-injecting fakes — BundleRepository (for bundle.go)
// ---------------------------------------------------------------------------

// errListBundleRepo overrides List to return an error.
type errListBundleRepo struct{ *fakeBundleRepo }

func (r *errListBundleRepo) List(_ context.Context) ([]domain.BundleEntry, error) {
	return nil, errors.New("list failed")
}

// errDeleteBundleRepo overrides Delete to return an error.
type errDeleteBundleRepo struct{ *fakeBundleRepo }

func (r *errDeleteBundleRepo) Delete(_ context.Context, _ string) error {
	return errors.New("delete failed")
}

// errPutBundleRepo overrides Put to return an error, so that ReindexBundle
// fails when it tries to stamp LastIndexedAt back to the repository.
type errPutBundleRepo struct{ *fakeBundleRepo }

func (r *errPutBundleRepo) Put(_ context.Context, _ domain.BundleEntry) error {
	return errors.New("put failed")
}

// ---------------------------------------------------------------------------
// SearchService error paths — FR-012, FR-013, FR-014
// ---------------------------------------------------------------------------

// TestSearchService_SemanticSearch_EmbedError covers the embed-failure branch
// in SemanticSearch (FR-012).
func TestSearchService_SemanticSearch_EmbedError(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    &errEmbedder{},
		VectorStore: newFakeVectorStore(),
	}
	_, err := svc.SemanticSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err == nil {
		t.Fatal("SemanticSearch: expected embed error, got nil")
	}
}

// TestSearchService_SemanticSearch_EmptyVecs covers the "embedder returned no
// vectors" guard in SemanticSearch (FR-012).
func TestSearchService_SemanticSearch_EmptyVecs(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    &emptyVecsEmbedder{},
		VectorStore: newFakeVectorStore(),
	}
	_, err := svc.SemanticSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err == nil {
		t.Fatal("SemanticSearch: expected empty-vecs error, got nil")
	}
}

// TestSearchService_SemanticSearch_StoreError covers the store.Search failure
// branch in SemanticSearch (FR-012).
func TestSearchService_SemanticSearch_StoreError(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: &errSearchVS{fakeVectorStore: newFakeVectorStore()},
	}
	_, err := svc.SemanticSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err == nil {
		t.Fatal("SemanticSearch: expected store error, got nil")
	}
}

// TestSearchService_KeywordSearch_EmbedError covers the embed-failure branch
// in KeywordSearch (FR-013).
func TestSearchService_KeywordSearch_EmbedError(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    &errEmbedder{},
		VectorStore: newFakeVectorStore(),
	}
	_, err := svc.KeywordSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err == nil {
		t.Fatal("KeywordSearch: expected embed error, got nil")
	}
}

// TestSearchService_KeywordSearch_EmptyVecs covers the "embedder returned no
// vectors" guard in KeywordSearch (FR-013).
func TestSearchService_KeywordSearch_EmptyVecs(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    &emptyVecsEmbedder{},
		VectorStore: newFakeVectorStore(),
	}
	_, err := svc.KeywordSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err == nil {
		t.Fatal("KeywordSearch: expected empty-vecs error, got nil")
	}
}

// TestSearchService_KeywordSearch_StoreError covers the store.Search failure
// branch in KeywordSearch (FR-013).
func TestSearchService_KeywordSearch_StoreError(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    newFakeEmbedder(4),
		VectorStore: &errSearchVS{fakeVectorStore: newFakeVectorStore()},
	}
	_, err := svc.KeywordSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5)
	if err == nil {
		t.Fatal("KeywordSearch: expected store error, got nil")
	}
}

// TestSearchService_RAGSearch_SemanticError covers the branch in RAGSearch
// where the underlying SemanticSearch fails (FR-014).
func TestSearchService_RAGSearch_SemanticError(t *testing.T) {
	t.Parallel()
	svc := &usecase.SearchService{
		Embedder:    &errEmbedder{},
		VectorStore: newFakeVectorStore(),
	}
	_, err := svc.RAGSearch(context.Background(), "query", domain.Scope{Kind: domain.ScopeGlobal}, 5, 0.0)
	if err == nil {
		t.Fatal("RAGSearch: expected error from SemanticSearch, got nil")
	}
}

// ---------------------------------------------------------------------------
// ConceptService error paths — FR-006, FR-007, FR-010, FR-011
// ---------------------------------------------------------------------------

// newConceptSvc constructs a ConceptService accepting any domain.NodeRepository
// so that error-injecting fakes can be substituted in place of *fakeNodeRepo.
func newConceptSvc(nr domain.NodeRepository) *usecase.ConceptService {
	return &usecase.ConceptService{
		NodeRepository:   nr,
		BundleRepository: newFakeBundleRepo(),
	}
}

// TestConceptService_ListConcepts_NonErrNotFoundError covers the error path in
// ListConcepts where the repository returns a non-ErrNotFound error (FR-006).
func TestConceptService_ListConcepts_NonErrNotFoundError(t *testing.T) {
	t.Parallel()
	nr := &errListNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	svc := newConceptSvc(nr)

	_, err := svc.ListConcepts(context.Background(), "kb", "")
	if err == nil {
		t.Fatal("ListConcepts: expected error from List, got nil")
	}
}

// TestConceptService_GetLinks_NotFound covers the error path in GetLinks when
// the concept does not exist (FR-007).
func TestConceptService_GetLinks_NotFound(t *testing.T) {
	t.Parallel()
	svc := newConceptSvc(newFakeNodeRepo())

	_, err := svc.GetLinks(
		context.Background(),
		domain.ConceptRef{BundleAlias: "kb", RelativePath: "ghost.md"},
	)
	if err == nil {
		t.Fatal("GetLinks: expected error for missing concept, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetLinks: expected ErrNotFound; got %v", err)
	}
}

// TestConceptService_ListTypes_Error covers the error path in ListTypes when
// the repository returns an error (FR-010).
func TestConceptService_ListTypes_Error(t *testing.T) {
	t.Parallel()
	nr := &errListTypesNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	svc := newConceptSvc(nr)

	_, err := svc.ListTypes(context.Background(), "kb")
	if err == nil {
		t.Fatal("ListTypes: expected error from ListTypes, got nil")
	}
}

// TestConceptService_WriteConcept_PutError covers the branch in WriteConcept
// where NodeRepository.Put fails (FR-011).
func TestConceptService_WriteConcept_PutError(t *testing.T) {
	t.Parallel()
	nr := &errPutNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	svc := newConceptSvc(nr)

	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/doc.md"}
	err := svc.WriteConcept(context.Background(), ref, makeConcept("note", "Doc", "body", nil))
	if err == nil {
		t.Fatal("WriteConcept: expected Put error, got nil")
	}
}

// TestConceptService_WriteConcept_RegenerateIndexError covers the branch in
// WriteConcept where regenerateIndex fails because List returns a non-ErrNotFound
// error (FR-011, regenerateIndex error path).
func TestConceptService_WriteConcept_RegenerateIndexError(t *testing.T) {
	t.Parallel()
	// Put succeeds (inherited fakeNodeRepo.Put); List always errors so that
	// regenerateIndex returns an error before WriteReserved is called.
	nr := &errListNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	svc := newConceptSvc(nr)

	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/doc.md"}
	err := svc.WriteConcept(context.Background(), ref, makeConcept("note", "Doc", "body", nil))
	if err == nil {
		t.Fatal("WriteConcept: expected regenerateIndex error, got nil")
	}
}

// TestConceptService_WriteConcept_AppendLogError covers the branch in
// WriteConcept where appendLog fails because ReadReserved returns a
// non-ErrNotFound error (FR-011, appendLog error path).
//
// regenerateIndex succeeds (List and WriteReserved delegate to working fakes);
// appendLog then calls ReadReserved which fails.
func TestConceptService_WriteConcept_AppendLogError(t *testing.T) {
	t.Parallel()
	// ReadReserved always returns a non-ErrNotFound error; Put, List, and
	// WriteReserved delegate to the embedded fakeNodeRepo.
	nr := &errReadReservedNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	svc := newConceptSvc(nr)

	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/doc.md"}
	err := svc.WriteConcept(context.Background(), ref, makeConcept("note", "Doc", "body", nil))
	if err == nil {
		t.Fatal("WriteConcept: expected appendLog error from ReadReserved, got nil")
	}
}

// ---------------------------------------------------------------------------
// BundleService error paths — FR-002, FR-003, FR-004
// ---------------------------------------------------------------------------

// TestBundleService_ListBundles_ListError covers the error path in ListBundles
// where BundleRepository.List fails (FR-003).
func TestBundleService_ListBundles_ListError(t *testing.T) {
	t.Parallel()
	br := &errListBundleRepo{fakeBundleRepo: newFakeBundleRepo()}
	svc := &usecase.BundleService{
		BundleRepository: br,
		NodeRepository:   newFakeNodeRepo(),
	}

	_, err := svc.ListBundles(context.Background())
	if err == nil {
		t.Fatal("ListBundles: expected List error, got nil")
	}
}

// TestBundleService_RemoveBundle_DeleteError covers the branch in RemoveBundle
// where BundleRepository.Delete fails (FR-002).
func TestBundleService_RemoveBundle_DeleteError(t *testing.T) {
	t.Parallel()
	br := &errDeleteBundleRepo{fakeBundleRepo: newFakeBundleRepo()}
	// Pre-seed via the embedded fake so Get succeeds.
	if err := br.Put(context.Background(), domain.BundleEntry{
		Alias:    "kb",
		RootPath: "/kb",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc := &usecase.BundleService{
		BundleRepository: br,
		NodeRepository:   newFakeNodeRepo(),
	}

	err := svc.RemoveBundle(context.Background(), "kb")
	if err == nil {
		t.Fatal("RemoveBundle: expected Delete error, got nil")
	}
}

// TestBundleService_ReindexBundle_ListError covers the branch in ReindexBundle
// where NodeRepository.List fails (FR-004).
func TestBundleService_ReindexBundle_ListError(t *testing.T) {
	t.Parallel()
	br := newFakeBundleRepo()
	if err := br.Put(context.Background(), domain.BundleEntry{
		Alias:    "kb",
		RootPath: "/kb",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	nr := &errListNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	svc := &usecase.BundleService{BundleRepository: br, NodeRepository: nr}

	err := svc.ReindexBundle(context.Background(), "kb", newFakeEmbedder(4), newFakeVectorStore())
	if err == nil {
		t.Fatal("ReindexBundle: expected List error, got nil")
	}
}

// TestBundleService_ReindexBundle_GetConceptError covers the branch in
// ReindexBundle where NodeRepository.Get fails for an individual concept (FR-004).
func TestBundleService_ReindexBundle_GetConceptError(t *testing.T) {
	t.Parallel()
	br := newFakeBundleRepo()
	if err := br.Put(context.Background(), domain.BundleEntry{
		Alias:    "kb",
		RootPath: "/kb",
	}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	// List (inherited) returns a ref; Get (overridden) fails.
	nr := &errGetNodeRepo{fakeNodeRepo: newFakeNodeRepo()}
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "a.md"}
	// Seed via inherited Put so List returns the ref.
	if err := nr.Put(context.Background(), ref, &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		Body:        "body",
	}); err != nil {
		t.Fatalf("seed concept: %v", err)
	}

	svc := &usecase.BundleService{BundleRepository: br, NodeRepository: nr}
	err := svc.ReindexBundle(context.Background(), "kb", newFakeEmbedder(4), newFakeVectorStore())
	if err == nil {
		t.Fatal("ReindexBundle: expected Get-concept error, got nil")
	}
}

// TestBundleService_ReindexBundle_EmbedError covers the branch in ReindexBundle
// where Embedder.Embed fails (FR-004).
func TestBundleService_ReindexBundle_EmbedError(t *testing.T) {
	t.Parallel()
	br := newFakeBundleRepo()
	if err := br.Put(context.Background(), domain.BundleEntry{
		Alias:    "kb",
		RootPath: "/kb",
	}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	nr := newFakeNodeRepo()
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "a.md"}
	if err := nr.Put(context.Background(), ref, &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		Body:        "body",
	}); err != nil {
		t.Fatalf("seed concept: %v", err)
	}

	svc := &usecase.BundleService{BundleRepository: br, NodeRepository: nr}
	// errEmbedder returns an error; since len(texts) > 0 the embed call is made.
	err := svc.ReindexBundle(context.Background(), "kb", &errEmbedder{}, newFakeVectorStore())
	if err == nil {
		t.Fatal("ReindexBundle: expected Embed error, got nil")
	}
}

// TestBundleService_ReindexBundle_UpsertError covers the branch in ReindexBundle
// where VectorStore.Upsert fails (FR-004).
func TestBundleService_ReindexBundle_UpsertError(t *testing.T) {
	t.Parallel()
	br := newFakeBundleRepo()
	if err := br.Put(context.Background(), domain.BundleEntry{
		Alias:    "kb",
		RootPath: "/kb",
	}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	nr := newFakeNodeRepo()
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "a.md"}
	if err := nr.Put(context.Background(), ref, &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		Body:        "body",
	}); err != nil {
		t.Fatalf("seed concept: %v", err)
	}

	vs := &errUpsertVS{fakeVectorStore: newFakeVectorStore()}
	svc := &usecase.BundleService{BundleRepository: br, NodeRepository: nr}
	err := svc.ReindexBundle(context.Background(), "kb", newFakeEmbedder(4), vs)
	if err == nil {
		t.Fatal("ReindexBundle: expected Upsert error, got nil")
	}
}

// TestBundleService_ReindexBundle_PutBundleError covers the branch in
// ReindexBundle where BundleRepository.Put fails when stamping LastIndexedAt
// (FR-004). An empty node repo skips embed/upsert and goes straight to Put.
func TestBundleService_ReindexBundle_PutBundleError(t *testing.T) {
	t.Parallel()
	br := &errPutBundleRepo{fakeBundleRepo: newFakeBundleRepo()}
	// Seed the bundle via the embedded fake to bypass the Put override.
	if err := br.fakeBundleRepo.Put(context.Background(), domain.BundleEntry{
		Alias:    "kb",
		RootPath: "/kb",
	}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	// Empty node repo → len(texts)==0 → embed/upsert skipped → Put is called.
	svc := &usecase.BundleService{
		BundleRepository: br,
		NodeRepository:   newFakeNodeRepo(),
	}
	err := svc.ReindexBundle(context.Background(), "kb", newFakeEmbedder(4), newFakeVectorStore())
	if err == nil {
		t.Fatal("ReindexBundle: expected Put-bundle error, got nil")
	}
}
