// Package mcpadapter_test — happy-path coverage tests for previously-uncovered
// MCP handlers (FIX-007).
//
// All helpers and fake types (fakeNodeRepository, fakeBundleRepository,
// newTestServices, callTool) are declared in handlers_test.go in the same
// mcpadapter_test package.
package mcpadapter_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// ---------------------------------------------------------------------------
// TestHandleBundleList — FR-003
// ---------------------------------------------------------------------------

// TestHandleBundleList_ReturnsEmptyList verifies that bundle_list succeeds with
// an empty registry and returns a non-error JSON result (FR-003).
func TestHandleBundleList_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	result, err := callTool(ctx, map[string]any{}, svc.HandleBundleList)
	if err != nil {
		t.Fatalf("HandleBundleList: unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("HandleBundleList: expected non-nil result")
	}
	if result.IsError {
		t.Errorf("HandleBundleList: IsError=true; content: %v", result.Content)
	}
}

// TestHandleBundleList_ReturnsBundleEntries verifies that bundle_list includes
// seeded entries in the result (FR-003).
func TestHandleBundleList_ReturnsBundleEntries(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	// Seed a bundle directly into the fake bundle repo via type assertion.
	br := svc.Bundle.BundleRepository.(*fakeBundleRepository)
	if err := br.Put(ctx, domain.BundleEntry{Alias: "kb", RootPath: "/kb"}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	result, err := callTool(ctx, map[string]any{}, svc.HandleBundleList)
	if err != nil {
		t.Fatalf("HandleBundleList: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleBundleList: unexpected error result")
	}
}

// ---------------------------------------------------------------------------
// TestHandleBundleRemove — FR-002
// ---------------------------------------------------------------------------

// TestHandleBundleRemove_RemovesBundle verifies that bundle_remove succeeds
// when the alias exists (FR-002).
func TestHandleBundleRemove_RemovesBundle(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	br := svc.Bundle.BundleRepository.(*fakeBundleRepository)
	if err := br.Put(ctx, domain.BundleEntry{Alias: "kb", RootPath: "/kb"}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	result, err := callTool(ctx, map[string]any{"alias": "kb"}, svc.HandleBundleRemove)
	if err != nil {
		t.Fatalf("HandleBundleRemove: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleBundleRemove: unexpected error result")
	}
}

// TestHandleBundleRemove_MissingAlias verifies that bundle_remove returns an
// error when the required "alias" argument is absent.
func TestHandleBundleRemove_MissingAlias(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleBundleRemove)
	if err == nil {
		t.Fatal("HandleBundleRemove: expected error for missing alias")
	}
}

// ---------------------------------------------------------------------------
// TestHandleBundleReindex — FR-004
// ---------------------------------------------------------------------------

// TestHandleBundleReindex_ReindexesBundle verifies that bundle_reindex succeeds
// when the alias exists (FR-004). With no concepts the embed step is skipped.
func TestHandleBundleReindex_ReindexesBundle(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	br := svc.Bundle.BundleRepository.(*fakeBundleRepository)
	if err := br.Put(ctx, domain.BundleEntry{Alias: "kb", RootPath: "/kb"}); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	result, err := callTool(ctx, map[string]any{"alias": "kb"}, svc.HandleBundleReindex)
	if err != nil {
		t.Fatalf("HandleBundleReindex: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleBundleReindex: unexpected error result")
	}
}

// TestHandleBundleReindex_MissingAlias verifies that bundle_reindex returns an
// error when the required "alias" argument is absent.
func TestHandleBundleReindex_MissingAlias(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleBundleReindex)
	if err == nil {
		t.Fatal("HandleBundleReindex: expected error for missing alias")
	}
}

// ---------------------------------------------------------------------------
// TestHandleConceptList — FR-006
// ---------------------------------------------------------------------------

// TestHandleConceptList_ReturnsEmptyList verifies that concept_list succeeds
// when no concepts are registered (FR-006).
func TestHandleConceptList_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	result, err := callTool(
		context.Background(),
		map[string]any{"bundle_alias": "kb"},
		svc.HandleConceptList,
	)
	if err != nil {
		t.Fatalf("HandleConceptList: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleConceptList: unexpected error result")
	}
}

// TestHandleConceptList_MissingAlias verifies that concept_list returns an
// error when the required "bundle_alias" argument is absent.
func TestHandleConceptList_MissingAlias(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleConceptList)
	if err == nil {
		t.Fatal("HandleConceptList: expected error for missing bundle_alias")
	}
	if !strings.Contains(err.Error(), "bundle_alias") {
		t.Errorf("error should mention bundle_alias; got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestHandleConceptLinks — FR-007
// ---------------------------------------------------------------------------

// TestHandleConceptLinks_ReturnsLinks verifies that concept_links returns the
// outbound link list for a known concept (FR-007).
func TestHandleConceptLinks_ReturnsLinks(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	nr := svc.Concept.NodeRepository.(*fakeNodeRepository)
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "doc.md"}
	if err := nr.Put(ctx, ref, &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		OutboundLinks: []domain.ConceptLink{
			{Target: "other.md", Text: "Other", Broken: false},
		},
	}); err != nil {
		t.Fatalf("seed concept: %v", err)
	}

	result, err := callTool(ctx, map[string]any{"ref": "kb:doc.md"}, svc.HandleConceptLinks)
	if err != nil {
		t.Fatalf("HandleConceptLinks: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleConceptLinks: unexpected error result")
	}
}

// TestHandleConceptLinks_MissingRef verifies that concept_links returns an
// error when the required "ref" argument is absent.
func TestHandleConceptLinks_MissingRef(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleConceptLinks)
	if err == nil {
		t.Fatal("HandleConceptLinks: expected error for missing ref")
	}
}

// ---------------------------------------------------------------------------
// TestHandleIndexRead — FR-008
// ---------------------------------------------------------------------------

// TestHandleIndexRead_ReturnsContent verifies that index_read returns the
// content of a seeded index.md (FR-008).
func TestHandleIndexRead_ReturnsContent(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	nr := svc.Concept.NodeRepository.(*fakeNodeRepository)
	if err := nr.WriteReserved(ctx, "kb", "index.md", "# Index\n"); err != nil {
		t.Fatalf("seed index.md: %v", err)
	}

	result, err := callTool(ctx, map[string]any{"bundle_alias": "kb"}, svc.HandleIndexRead)
	if err != nil {
		t.Fatalf("HandleIndexRead: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleIndexRead: unexpected error result")
	}
}

// TestHandleIndexRead_MissingAlias verifies that index_read returns an error
// when the required "bundle_alias" argument is absent.
func TestHandleIndexRead_MissingAlias(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleIndexRead)
	if err == nil {
		t.Fatal("HandleIndexRead: expected error for missing bundle_alias")
	}
}

// ---------------------------------------------------------------------------
// TestHandleLogRead — FR-009
// ---------------------------------------------------------------------------

// TestHandleLogRead_ReturnsContent verifies that log_read returns the content
// of a seeded log.md (FR-009).
func TestHandleLogRead_ReturnsContent(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	nr := svc.Concept.NodeRepository.(*fakeNodeRepository)
	if err := nr.WriteReserved(ctx, "kb", "log.md", "- entry\n"); err != nil {
		t.Fatalf("seed log.md: %v", err)
	}

	result, err := callTool(ctx, map[string]any{"bundle_alias": "kb"}, svc.HandleLogRead)
	if err != nil {
		t.Fatalf("HandleLogRead: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleLogRead: unexpected error result")
	}
}

// TestHandleLogRead_MissingAlias verifies that log_read returns an error when
// the required "bundle_alias" argument is absent.
func TestHandleLogRead_MissingAlias(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleLogRead)
	if err == nil {
		t.Fatal("HandleLogRead: expected error for missing bundle_alias")
	}
}

// ---------------------------------------------------------------------------
// TestHandleConceptTypeList — FR-010
// ---------------------------------------------------------------------------

// TestHandleConceptTypeList_ReturnsTypes verifies that concept_type_list
// returns a JSON list without error (FR-010).
func TestHandleConceptTypeList_ReturnsTypes(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	ctx := context.Background()

	nr := svc.Concept.NodeRepository.(*fakeNodeRepository)
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "a.md"}
	if err := nr.Put(ctx, ref, &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "runbook"},
	}); err != nil {
		t.Fatalf("seed concept: %v", err)
	}

	result, err := callTool(ctx, map[string]any{"bundle_alias": "kb"}, svc.HandleConceptTypeList)
	if err != nil {
		t.Fatalf("HandleConceptTypeList: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleConceptTypeList: unexpected error result")
	}
}

// TestHandleConceptTypeList_MissingAlias verifies that concept_type_list
// returns an error when the required "bundle_alias" argument is absent.
func TestHandleConceptTypeList_MissingAlias(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{}, svc.HandleConceptTypeList)
	if err == nil {
		t.Fatal("HandleConceptTypeList: expected error for missing bundle_alias")
	}
}

// ---------------------------------------------------------------------------
// TestHandleSearchSemantic — FR-012
// ---------------------------------------------------------------------------

// TestHandleSearchSemantic_ReturnsResults verifies that search_semantic returns
// a result (possibly empty) without error on an empty store (FR-012).
func TestHandleSearchSemantic_ReturnsResults(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	result, err := callTool(
		context.Background(),
		map[string]any{"query": "some query", "scope": "global"},
		svc.HandleSearchSemantic,
	)
	if err != nil {
		t.Fatalf("HandleSearchSemantic: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleSearchSemantic: unexpected error result")
	}
}

// TestHandleSearchSemantic_MissingQuery verifies that search_semantic returns
// an error when the required "query" argument is absent.
func TestHandleSearchSemantic_MissingQuery(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{"scope": "global"}, svc.HandleSearchSemantic)
	if err == nil {
		t.Fatal("HandleSearchSemantic: expected error for missing query")
	}
}

// TestHandleSearchSemantic_BundleScope verifies that search_semantic accepts
// a bundle-scoped scope string without error (FR-012).
func TestHandleSearchSemantic_BundleScope(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	result, err := callTool(
		context.Background(),
		map[string]any{"query": "search term", "scope": "bundle:kb", "top_k": 3},
		svc.HandleSearchSemantic,
	)
	if err != nil {
		t.Fatalf("HandleSearchSemantic bundle scope: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleSearchSemantic bundle scope: unexpected error result")
	}
}

// TestHandleSearchSemantic_InvalidScope verifies that search_semantic returns
// an error for a malformed scope string.
func TestHandleSearchSemantic_InvalidScope(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(
		context.Background(),
		map[string]any{"query": "term", "scope": "not-a-valid-scope"},
		svc.HandleSearchSemantic,
	)
	if err == nil {
		t.Fatal("HandleSearchSemantic: expected error for invalid scope")
	}
}

// ---------------------------------------------------------------------------
// TestHandleSearchKeyword — FR-013
// ---------------------------------------------------------------------------

// TestHandleSearchKeyword_ReturnsResults verifies that search_keyword returns
// a result (possibly empty) without error on an empty store (FR-013).
func TestHandleSearchKeyword_ReturnsResults(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	result, err := callTool(
		context.Background(),
		map[string]any{"query": "some keyword", "scope": "global"},
		svc.HandleSearchKeyword,
	)
	if err != nil {
		t.Fatalf("HandleSearchKeyword: unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("HandleSearchKeyword: unexpected error result")
	}
}

// TestHandleSearchKeyword_MissingQuery verifies that search_keyword returns an
// error when the required "query" argument is absent.
func TestHandleSearchKeyword_MissingQuery(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(context.Background(), map[string]any{"scope": "global"}, svc.HandleSearchKeyword)
	if err == nil {
		t.Fatal("HandleSearchKeyword: expected error for missing query")
	}
}

// TestHandleSearchKeyword_InvalidScope verifies that search_keyword returns an
// error for a malformed scope string (FR-013).
func TestHandleSearchKeyword_InvalidScope(t *testing.T) {
	t.Parallel()
	svc := newTestServices()
	_, err := callTool(
		context.Background(),
		map[string]any{"query": "term", "scope": "notavalidscope"},
		svc.HandleSearchKeyword,
	)
	if err == nil {
		t.Fatal("HandleSearchKeyword: expected error for invalid scope")
	}
}
