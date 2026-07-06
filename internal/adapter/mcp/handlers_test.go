package mcpadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	mcpadapter "github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/mcp"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
)

// ---------------------------------------------------------------------------
// In-file fakes for domain interfaces
// (domain_test package fakes are test-only and not importable)
// ---------------------------------------------------------------------------

type fakeEmbedder struct{ dims int }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, f.dims)
	}
	return out, nil
}
func (f *fakeEmbedder) Dims() int { return f.dims }

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
func (f *fakeVectorStore) Search(_ context.Context, _ []float32, scope domain.Scope, topK int) ([]domain.ScoredChunk, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var results []domain.ScoredChunk
	for _, c := range f.chunks {
		if !chunkInScope(c, scope) {
			continue
		}
		results = append(results, domain.ScoredChunk{
			Source:     c.BundleAlias + ":" + c.ConceptPath,
			ChunkIndex: c.ChunkIndex,
			ChunkText:  c.Text,
			Score:      1.0,
		})
		if len(results) >= topK {
			break
		}
	}
	return results, nil
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

func chunkInScope(c domain.EmbeddingChunk, scope domain.Scope) bool {
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

type fakeNodeRepository struct {
	mu       sync.RWMutex
	concepts map[string]*domain.OKFConcept
	reserved map[string]string
}

func newFakeNodeRepository() *fakeNodeRepository {
	return &fakeNodeRepository{
		concepts: make(map[string]*domain.OKFConcept),
		reserved: make(map[string]string),
	}
}
func (f *fakeNodeRepository) Get(_ context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.concepts[ref.String()]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}
func (f *fakeNodeRepository) Put(_ context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.concepts[ref.String()] = concept
	return nil
}
func (f *fakeNodeRepository) List(_ context.Context, bundleAlias, subPath string) ([]domain.ConceptRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	prefix := bundleAlias + ":"
	if subPath != "" {
		prefix = bundleAlias + ":" + subPath
	}
	var refs []domain.ConceptRef
	for key := range f.concepts {
		if strings.HasPrefix(key, prefix) {
			relPath := strings.TrimPrefix(key, bundleAlias+":")
			refs = append(refs, domain.ConceptRef{BundleAlias: bundleAlias, RelativePath: relPath})
		}
	}
	return refs, nil
}
func (f *fakeNodeRepository) ListTypes(_ context.Context, bundleAlias string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	prefix := bundleAlias + ":"
	seen := make(map[string]struct{})
	for key, c := range f.concepts {
		if strings.HasPrefix(key, prefix) {
			if t := c.Frontmatter.Type; t != "" {
				seen[t] = struct{}{}
			}
		}
	}
	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	return types, nil
}
func (f *fakeNodeRepository) ReadReserved(_ context.Context, bundleAlias, relPath string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	content, ok := f.reserved[bundleAlias+":"+relPath]
	if !ok {
		return "", domain.ErrNotFound
	}
	return content, nil
}
func (f *fakeNodeRepository) WriteReserved(_ context.Context, bundleAlias, relPath, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserved[bundleAlias+":"+relPath] = content
	return nil
}

type fakeBundleRepository struct {
	mu      sync.RWMutex
	bundles map[string]domain.BundleEntry
}

func newFakeBundleRepository() *fakeBundleRepository {
	return &fakeBundleRepository{bundles: make(map[string]domain.BundleEntry)}
}
func (f *fakeBundleRepository) Get(_ context.Context, alias string) (*domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	entry, ok := f.bundles[alias]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &entry, nil
}
func (f *fakeBundleRepository) Put(_ context.Context, entry domain.BundleEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[entry.Alias] = entry
	return nil
}
func (f *fakeBundleRepository) Delete(_ context.Context, alias string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bundles, alias)
	return nil
}
func (f *fakeBundleRepository) List(_ context.Context) ([]domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	entries := make([]domain.BundleEntry, 0, len(f.bundles))
	for _, e := range f.bundles {
		entries = append(entries, e)
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// Test helper — build a minimal Services with in-memory fakes.
// ---------------------------------------------------------------------------

func newTestServices() mcpadapter.Services {
	bundleRepo := newFakeBundleRepository()
	nodeRepo := newFakeNodeRepository()
	embedder := &fakeEmbedder{dims: 4}
	store := newFakeVectorStore()

	return mcpadapter.Services{
		Bundle: &usecase.BundleService{
			BundleRepository: bundleRepo,
			NodeRepository:   nodeRepo,
		},
		Concept: &usecase.ConceptService{
			NodeRepository:   nodeRepo,
			BundleRepository: bundleRepo,
		},
		Search: &usecase.SearchService{
			Embedder:    embedder,
			VectorStore: store,
		},
		Embedder:    embedder,
		VectorStore: store,
	}
}

// callTool builds a CallToolRequest from a map[string]any and calls handler.
func callTool(ctx context.Context, args map[string]any, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
	return handler(ctx, req)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHandleBundleAdd_RejectsMissingAlias verifies that bundle_add returns an
// error when the required "alias" argument is absent (FR-020, adapter
// boundary validation).
func TestHandleBundleAdd_RejectsMissingAlias(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"path": "/some/path",
		// alias deliberately omitted
	}, svc.HandleBundleAdd)

	if err == nil {
		t.Fatal("expected error when alias is missing, got nil")
	}
	if !strings.Contains(err.Error(), "alias") {
		t.Errorf("error message should mention \"alias\"; got: %v", err)
	}
}

// TestHandleBundleAdd_RejectsColonInAlias_FIX006 verifies that bundle_add
// rejects an alias containing ':' with a specific error before path validation.
// A colon in the alias breaks parseConceptRef for all concepts in that bundle,
// making them silently inaccessible via MCP tools.
func TestHandleBundleAdd_RejectsColonInAlias_FIX006(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	// Use a real temp directory with an .md file so path validation passes.
	// This isolates the alias colon check from the path-not-found check.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("# hi"), 0o644); err != nil { //nolint:gosec // test helper
		t.Fatal(err)
	}

	_, err := callTool(ctx, map[string]any{
		"alias": "bad:alias",
		"path":  dir,
	}, svc.HandleBundleAdd)

	if err == nil {
		t.Fatal("expected error for alias containing ':', got nil")
	}
	// The error must specifically mention the colon constraint, not a path error.
	if !strings.Contains(err.Error(), ":") || strings.Contains(err.Error(), "stat") {
		t.Errorf("expected alias-colon error; got: %v", err)
	}
}

// TestHandleConceptWrite_RejectsMissingType verifies that concept_write returns
// an error when the required "type" frontmatter field is absent.
// Spec: FR-011 — "Rejects if frontmatter type field is missing or empty."
func TestHandleConceptWrite_RejectsMissingType(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"ref":  "kb:notes/idea.md",
		"body": "Some body content.",
		// type deliberately omitted
	}, svc.HandleConceptWrite)

	if err == nil {
		t.Fatal("expected error when type is missing, got nil")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("error message should mention \"type\"; got: %v", err)
	}
}

// TestHandleConceptWrite_RejectsOversizedBody_FR020 verifies that concept_write
// returns a domain.ErrInputTooLarge error before the use case is invoked when
// the body exceeds MaxBodyBytes (1 MB). This directly tests FR-020.
func TestHandleConceptWrite_RejectsOversizedBody_FR020(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	oversizedBody := strings.Repeat("x", mcpadapter.MaxBodyBytes+1)

	_, err := callTool(ctx, map[string]any{
		"ref":  "kb:notes/big.md",
		"type": "note",
		"body": oversizedBody,
	}, svc.HandleConceptWrite)

	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
	if !errors.Is(err, domain.ErrInputTooLarge) {
		t.Errorf("expected domain.ErrInputTooLarge; got: %v", err)
	}
}

// TestHandleSearchRAG_ScopeGlobal verifies that search_rag with scope "global"
// returns a result (possibly empty) without error when the vector store is
// empty. Spec: FR-014 — "Returns empty list if no chunks meet threshold."
func TestHandleSearchRAG_ScopeGlobal(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{
		"query": "anything",
		"scope": "global",
		"top_k": 5,
	}, svc.HandleSearchRAG)

	if err != nil {
		t.Fatalf("expected no error for global scope search on empty store; got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil CallToolResult, got nil")
	}
	if result.IsError {
		t.Errorf("expected IsError=false, got IsError=true; content: %v", result.Content)
	}
}

// TestHandlePath_RejectsTraversal_FR019 verifies that any handler receiving a
// concept ref containing ".." path components returns a domain.ErrPathEscape
// error without reading or writing any file. Spec: FR-019.
func TestHandlePath_RejectsTraversal_FR019(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	traversalRefs := []string{
		"kb:../../etc/passwd",
		"kb:notes/../../../secret.md",
		"kb:../escape.md",
	}

	for _, ref := range traversalRefs {
		t.Run(ref, func(t *testing.T) {
			_, err := callTool(ctx, map[string]any{
				"ref": ref,
			}, svc.HandleConceptRead)

			if err == nil {
				t.Fatalf("expected path traversal error for ref %q, got nil", ref)
			}
			if !errors.Is(err, domain.ErrPathEscape) {
				t.Errorf("expected domain.ErrPathEscape for ref %q; got: %v", ref, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional handler tests for coverage uplift (FR-R04)
// ---------------------------------------------------------------------------

// TestHandleBundleList_Empty verifies bundle_list returns an empty result
// when no bundles are registered.
func TestHandleBundleList_Empty(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{}, svc.HandleBundleList)
	if err != nil {
		t.Fatalf("HandleBundleList: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("HandleBundleList: expected non-error result, got %v", result)
	}
}

// TestHandleConceptRead_NotFound verifies concept_read returns an error
// for a concept that does not exist.
func TestHandleConceptRead_NotFound(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"ref": "kb:missing.md",
	}, svc.HandleConceptRead)
	if err == nil {
		t.Fatal("expected error for missing concept, got nil")
	}
}

// TestHandleConceptRead_InvalidRef verifies concept_read rejects a malformed ref.
func TestHandleConceptRead_InvalidRef(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"ref": "notacolon",
	}, svc.HandleConceptRead)
	if err == nil {
		t.Fatal("expected error for invalid ref, got nil")
	}
}

// TestHandleConceptWrite_HappyPath verifies concept_write creates a concept.
func TestHandleConceptWrite_HappyPath(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{
		"ref":  "kb:notes/hello.md",
		"type": "note",
		"body": "# Hello",
	}, svc.HandleConceptWrite)
	if err != nil {
		t.Fatalf("HandleConceptWrite happy path: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected non-error result")
	}
}

// TestHandleConceptList_Empty verifies concept_list returns empty for unknown bundle.
func TestHandleConceptList_Empty(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{
		"bundle_alias": "noexist",
	}, svc.HandleConceptList)
	if err != nil {
		t.Fatalf("HandleConceptList: unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestHandleConceptLinks_NotFound verifies concept_links returns an error for
// a non-existent concept.
func TestHandleConceptLinks_NotFound(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"ref": "kb:ghost.md",
	}, svc.HandleConceptLinks)
	if err == nil {
		t.Fatal("expected error for missing concept, got nil")
	}
}

// TestHandleIndexRead_NotFound verifies index_read returns an error when
// index.md does not exist.
func TestHandleIndexRead_NotFound(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"bundle_alias": "kb",
		"dir_path":     "",
	}, svc.HandleIndexRead)
	if err == nil {
		t.Fatal("expected ErrNotFound for missing index.md, got nil")
	}
}

// TestHandleLogRead_NotFound verifies log_read returns an error when
// log.md does not exist.
func TestHandleLogRead_NotFound(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"bundle_alias": "kb",
		"dir_path":     "",
	}, svc.HandleLogRead)
	if err == nil {
		t.Fatal("expected ErrNotFound for missing log.md, got nil")
	}
}

// TestHandleConceptTypeList_Empty verifies concept_type_list returns an empty
// list for a bundle with no typed concepts.
func TestHandleConceptTypeList_Empty(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{
		"bundle_alias": "kb",
	}, svc.HandleConceptTypeList)
	if err != nil {
		t.Fatalf("HandleConceptTypeList: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected non-error result")
	}
}

// TestHandleSearchSemantic_EmptyStore verifies search_semantic returns
// empty results on an empty vector store.
func TestHandleSearchSemantic_EmptyStore(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{
		"query": "anything",
		"scope": "global",
		"top_k": 5,
	}, svc.HandleSearchSemantic)
	if err != nil {
		t.Fatalf("HandleSearchSemantic: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected non-error result")
	}
}

// TestHandleSearchKeyword_EmptyStore verifies search_keyword returns
// empty results on an empty vector store.
func TestHandleSearchKeyword_EmptyStore(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{
		"query": "anything",
		"scope": "global",
		"top_k": 5,
	}, svc.HandleSearchKeyword)
	if err != nil {
		t.Fatalf("HandleSearchKeyword: %v", err)
	}
	if result == nil || result.IsError {
		t.Errorf("expected non-error result")
	}
}

// TestHandleBundleRemove_NotFound verifies bundle_remove returns an error
// for an unknown alias.
func TestHandleBundleRemove_NotFound(t *testing.T) {
	svc := newTestServices()
	ctx := context.Background()

	_, err := callTool(ctx, map[string]any{
		"alias": "notregistered",
	}, svc.HandleBundleRemove)
	if err == nil {
		t.Fatal("expected error removing unknown bundle, got nil")
	}
}
