package mcpadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
)

// Services aggregates all use-case services required by the MCP tool handlers.
// Construct this in the DI wiring (cmd/tahu/main.go) and pass it to
// RegisterTools.
type Services struct {
	Bundle      *usecase.BundleService
	Concept     *usecase.ConceptService
	Search      *usecase.SearchService
	Embedder    domain.Embedder    // used by HandleBundleReindex
	VectorStore domain.VectorStore // used by HandleBundleReindex
}

// ---------------------------------------------------------------------------
// Bundle management (FR-001 through FR-004)
// ---------------------------------------------------------------------------

// HandleBundleList returns alias, root_path, concept_count, and last_indexed_at
// for every registered bundle (FR-003).
func (s *Services) HandleBundleList(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bundles, err := s.Bundle.ListBundles(ctx)
	if err != nil {
		return nil, fmt.Errorf("bundle_list: %w", err)
	}
	result, err := mcp.NewToolResultJSON(bundles)
	if err != nil {
		return nil, fmt.Errorf("bundle_list: serialize: %w", err)
	}
	return result, nil
}

// HandleBundleAdd registers an OKF bundle by filesystem path and alias
// (FR-001). Required arguments: alias, path. Optional: description, tags.
func (s *Services) HandleBundleAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("alias")
	if err != nil {
		return nil, fmt.Errorf("bundle_add: %w", err)
	}
	if err := ValidateInputSize(alias, MaxStringBytes); err != nil {
		return nil, fmt.Errorf("bundle_add: alias: %w", err)
	}

	rootPath, err := req.RequireString("path")
	if err != nil {
		return nil, fmt.Errorf("bundle_add: %w", err)
	}
	if err := ValidateInputSize(rootPath, MaxStringBytes); err != nil {
		return nil, fmt.Errorf("bundle_add: path: %w", err)
	}

	description := req.GetString("description", "")
	tags := req.GetStringSlice("tags", nil)

	entry, err := s.Bundle.AddBundle(ctx, alias, rootPath, description, tags)
	if err != nil {
		return nil, fmt.Errorf("bundle_add: %w", err)
	}

	result, err := mcp.NewToolResultJSON(entry)
	if err != nil {
		return nil, fmt.Errorf("bundle_add: serialize: %w", err)
	}
	return result, nil
}

// HandleBundleRemove unregisters a bundle by alias without deleting files
// (FR-002). Required argument: alias.
func (s *Services) HandleBundleRemove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("alias")
	if err != nil {
		return nil, fmt.Errorf("bundle_remove: %w", err)
	}
	if err := s.Bundle.RemoveBundle(ctx, alias); err != nil {
		return nil, fmt.Errorf("bundle_remove: %w", err)
	}
	return mcp.NewToolResultText("bundle removed: " + alias), nil
}

// HandleBundleReindex forces a full re-embed and reindex of a bundle (FR-004).
// Required argument: alias.
func (s *Services) HandleBundleReindex(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("alias")
	if err != nil {
		return nil, fmt.Errorf("bundle_reindex: %w", err)
	}
	if err := s.Bundle.ReindexBundle(ctx, alias, s.Embedder, s.VectorStore); err != nil {
		return nil, fmt.Errorf("bundle_reindex: %w", err)
	}
	return mcp.NewToolResultText("bundle reindexed: " + alias), nil
}

// ---------------------------------------------------------------------------
// OKF read / navigate (FR-005 through FR-010)
// ---------------------------------------------------------------------------

// HandleConceptRead returns parsed frontmatter + body for a concept (FR-005).
// Required argument: ref in "alias:relative/path.md" format.
func (s *Services) HandleConceptRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	refStr, err := req.RequireString("ref")
	if err != nil {
		return nil, fmt.Errorf("concept_read: %w", err)
	}
	ref, err := parseConceptRef(refStr)
	if err != nil {
		return nil, fmt.Errorf("concept_read: %w", err)
	}
	if err := ValidatePath(ref.BundleAlias, ref.RelativePath); err != nil {
		return nil, fmt.Errorf("concept_read: %w", err)
	}
	concept, err := s.Concept.ReadConcept(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("concept_read: %w", err)
	}
	result, err := mcp.NewToolResultJSON(concept)
	if err != nil {
		return nil, fmt.Errorf("concept_read: serialize: %w", err)
	}
	return result, nil
}

// HandleConceptWrite creates or updates an OKF concept document (FR-011).
//
// Required arguments: ref ("alias:relative/path.md"), type.
// Optional: title, description, resource, tags, body.
//
// Adapter boundary enforcement (FR-020):
//   - body ≤ MaxBodyBytes (1 MB) checked before the use-case call.
//   - all other string inputs ≤ MaxStringBytes (4 KB).
func (s *Services) HandleConceptWrite(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	refStr, err := req.RequireString("ref")
	if err != nil {
		return nil, fmt.Errorf("concept_write: %w", err)
	}
	ref, err := parseConceptRef(refStr)
	if err != nil {
		return nil, fmt.Errorf("concept_write: %w", err)
	}
	if err := ValidatePath(ref.BundleAlias, ref.RelativePath); err != nil {
		return nil, fmt.Errorf("concept_write: %w", err)
	}

	// Validate body size before invoking the use case (FR-020).
	body := req.GetString("body", "")
	if err := ValidateInputSize(body, MaxBodyBytes); err != nil {
		return nil, fmt.Errorf("concept_write: %w", err)
	}

	conceptType, err := req.RequireString("type")
	if err != nil {
		return nil, fmt.Errorf("concept_write: %w", err)
	}
	if err := ValidateInputSize(conceptType, MaxStringBytes); err != nil {
		return nil, fmt.Errorf("concept_write: type: %w", err)
	}

	title := req.GetString("title", "")
	description := req.GetString("description", "")
	resource := req.GetString("resource", "")
	tags := req.GetStringSlice("tags", nil)

	fm := domain.OKFFrontmatter{
		Type:        conceptType,
		Title:       title,
		Description: description,
		Resource:    resource,
		Tags:        tags,
	}
	concept := &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: fm,
		Body:        body,
	}

	if err := s.Concept.WriteConcept(ctx, ref, concept); err != nil {
		return nil, fmt.Errorf("concept_write: %w", err)
	}
	return mcp.NewToolResultText("concept written: " + ref.String()), nil
}

// HandleConceptList returns all non-reserved .md files at a directory level
// (FR-006). Required argument: bundle_alias. Optional: sub_path.
func (s *Services) HandleConceptList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("bundle_alias")
	if err != nil {
		return nil, fmt.Errorf("concept_list: %w", err)
	}
	subPath := req.GetString("sub_path", "")
	if subPath != "" {
		if err := ValidatePath(alias, subPath); err != nil {
			return nil, fmt.Errorf("concept_list: %w", err)
		}
	}
	refs, err := s.Concept.ListConcepts(ctx, alias, subPath)
	if err != nil {
		return nil, fmt.Errorf("concept_list: %w", err)
	}
	result, err := mcp.NewToolResultJSON(refs)
	if err != nil {
		return nil, fmt.Errorf("concept_list: serialize: %w", err)
	}
	return result, nil
}

// HandleConceptLinks returns all outbound markdown hyperlink targets from a
// concept body (FR-007). Required argument: ref.
// Broken links are included with Broken set to true.
func (s *Services) HandleConceptLinks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	refStr, err := req.RequireString("ref")
	if err != nil {
		return nil, fmt.Errorf("concept_links: %w", err)
	}
	ref, err := parseConceptRef(refStr)
	if err != nil {
		return nil, fmt.Errorf("concept_links: %w", err)
	}
	if err := ValidatePath(ref.BundleAlias, ref.RelativePath); err != nil {
		return nil, fmt.Errorf("concept_links: %w", err)
	}
	links, err := s.Concept.GetLinks(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("concept_links: %w", err)
	}
	result, err := mcp.NewToolResultJSON(links)
	if err != nil {
		return nil, fmt.Errorf("concept_links: serialize: %w", err)
	}
	return result, nil
}

// HandleIndexRead returns the raw content of index.md at a directory level
// (FR-008). Required argument: bundle_alias. Optional: dir_path.
func (s *Services) HandleIndexRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("bundle_alias")
	if err != nil {
		return nil, fmt.Errorf("index_read: %w", err)
	}
	dirPath := req.GetString("dir_path", "")
	if dirPath != "" {
		if err := ValidatePath(alias, dirPath); err != nil {
			return nil, fmt.Errorf("index_read: %w", err)
		}
	}
	content, err := s.Concept.ReadIndex(ctx, alias, dirPath)
	if err != nil {
		return nil, fmt.Errorf("index_read: %w", err)
	}
	return mcp.NewToolResultText(content), nil
}

// HandleLogRead returns the raw content of log.md at a directory level
// (FR-009). Required argument: bundle_alias. Optional: dir_path.
func (s *Services) HandleLogRead(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("bundle_alias")
	if err != nil {
		return nil, fmt.Errorf("log_read: %w", err)
	}
	dirPath := req.GetString("dir_path", "")
	if dirPath != "" {
		if err := ValidatePath(alias, dirPath); err != nil {
			return nil, fmt.Errorf("log_read: %w", err)
		}
	}
	content, err := s.Concept.ReadLog(ctx, alias, dirPath)
	if err != nil {
		return nil, fmt.Errorf("log_read: %w", err)
	}
	return mcp.NewToolResultText(content), nil
}

// HandleConceptTypeList returns all distinct frontmatter type values in a
// bundle (FR-010). Required argument: bundle_alias.
func (s *Services) HandleConceptTypeList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	alias, err := req.RequireString("bundle_alias")
	if err != nil {
		return nil, fmt.Errorf("concept_type_list: %w", err)
	}
	types, err := s.Concept.ListTypes(ctx, alias)
	if err != nil {
		return nil, fmt.Errorf("concept_type_list: %w", err)
	}
	result, err := mcp.NewToolResultJSON(types)
	if err != nil {
		return nil, fmt.Errorf("concept_type_list: serialize: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Search (FR-012 through FR-014)
// ---------------------------------------------------------------------------

// HandleSearchSemantic returns a ranked list of chunks via vector similarity
// (FR-012). Required arguments: query, scope.
// Optional: top_k (default 10).
func (s *Services) HandleSearchSemantic(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return nil, fmt.Errorf("search_semantic: %w", err)
	}
	if err := ValidateInputSize(query, MaxStringBytes); err != nil {
		return nil, fmt.Errorf("search_semantic: query: %w", err)
	}

	scopeStr := req.GetString("scope", "global")
	scope, err := ParseScope(scopeStr)
	if err != nil {
		return nil, fmt.Errorf("search_semantic: scope: %w", err)
	}

	topK := req.GetInt("top_k", 10)
	if topK < 1 {
		topK = 1
	}

	chunks, err := s.Search.SemanticSearch(ctx, query, scope, topK)
	if err != nil {
		return nil, fmt.Errorf("search_semantic: %w", err)
	}
	result, err := mcp.NewToolResultJSON(chunks)
	if err != nil {
		return nil, fmt.Errorf("search_semantic: serialize: %w", err)
	}
	return result, nil
}

// HandleSearchKeyword returns a ranked list of chunks via Okapi BM25 (FR-013).
// Required arguments: query, scope. Optional: top_k (default 10).
func (s *Services) HandleSearchKeyword(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return nil, fmt.Errorf("search_keyword: %w", err)
	}
	if err := ValidateInputSize(query, MaxStringBytes); err != nil {
		return nil, fmt.Errorf("search_keyword: query: %w", err)
	}

	scopeStr := req.GetString("scope", "global")
	scope, err := ParseScope(scopeStr)
	if err != nil {
		return nil, fmt.Errorf("search_keyword: scope: %w", err)
	}

	topK := req.GetInt("top_k", 10)
	if topK < 1 {
		topK = 1
	}

	chunks, err := s.Search.KeywordSearch(ctx, query, scope, topK)
	if err != nil {
		return nil, fmt.Errorf("search_keyword: %w", err)
	}
	result, err := mcp.NewToolResultJSON(chunks)
	if err != nil {
		return nil, fmt.Errorf("search_keyword: serialize: %w", err)
	}
	return result, nil
}

// HandleSearchRAG performs semantic search filtered by score (FR-014).
//
// Required arguments: query. Optional: scope (default "global"),
// top_k (default 5, max 20), min_score (default 0.0).
//
// Returns up to top_k chunks with score >= min_score in descending order.
// Returns an empty list when no chunks meet the threshold; this is not an
// error.
func (s *Services) HandleSearchRAG(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return nil, fmt.Errorf("search_rag: %w", err)
	}
	if err := ValidateInputSize(query, MaxStringBytes); err != nil {
		return nil, fmt.Errorf("search_rag: query: %w", err)
	}

	scopeStr := req.GetString("scope", "global")
	scope, err := ParseScope(scopeStr)
	if err != nil {
		return nil, fmt.Errorf("search_rag: scope: %w", err)
	}

	topK := req.GetInt("top_k", 5)
	if topK < 1 {
		topK = 1
	}
	if topK > 20 {
		topK = 20
	}

	minScore := float32(req.GetFloat("min_score", 0.0))

	chunks, err := s.Search.RAGSearch(ctx, query, scope, topK, minScore)
	if err != nil {
		return nil, fmt.Errorf("search_rag: %w", err)
	}
	result, err := mcp.NewToolResultJSON(chunks)
	if err != nil {
		return nil, fmt.Errorf("search_rag: serialize: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// parseConceptRef parses a concept reference string in "alias:relative/path.md"
// format into a domain.ConceptRef.
func parseConceptRef(ref string) (domain.ConceptRef, error) {
	idx := strings.Index(ref, ":")
	if idx <= 0 {
		return domain.ConceptRef{}, fmt.Errorf(
			"invalid concept ref %q: expected format alias:relative/path.md", ref)
	}
	return domain.ConceptRef{
		BundleAlias:  ref[:idx],
		RelativePath: ref[idx+1:],
	}, nil
}
