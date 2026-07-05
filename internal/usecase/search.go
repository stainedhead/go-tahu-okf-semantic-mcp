// Package usecase implements application-level workflows for the tahu daemon.
// It depends only on domain interfaces and never imports adapter or infra
// packages, keeping the Clean Architecture dependency rule intact.
package usecase

import (
	"context"
	"fmt"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// SearchService implements semantic, keyword, and RAG search over OKF bundles.
// It composes a domain.Embedder (for query encoding) with a domain.VectorStore
// (for nearest-neighbour retrieval).
//
// The BM25 embedder also satisfies domain.Embedder, so KeywordSearch uses the
// same call path as SemanticSearch — the difference lies in which concrete
// Embedder is injected, not in the use-case logic.
type SearchService struct {
	Embedder    domain.Embedder
	VectorStore domain.VectorStore
}

// SemanticSearch returns up to topK chunks most similar to query, filtered by
// scope. The query is encoded with the dense (vector) embedder, then the
// resulting vector is passed to VectorStore.Search. Implements FR-012.
func (s *SearchService) SemanticSearch(
	ctx context.Context,
	query string,
	scope domain.Scope,
	topK int,
) ([]domain.ScoredChunk, error) {
	vecs, err := s.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("search.SemanticSearch embed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("search.SemanticSearch: embedder returned no vectors for one input")
	}
	chunks, err := s.VectorStore.Search(ctx, vecs[0], scope, topK)
	if err != nil {
		return nil, fmt.Errorf("search.SemanticSearch store: %w", err)
	}
	return chunks, nil
}

// KeywordSearch returns up to topK chunks ranked by keyword relevance (BM25).
// The BM25 embedder implements domain.Embedder so the call path is identical
// to SemanticSearch — the distinction is which Embedder is wired in.
// Implements FR-013.
func (s *SearchService) KeywordSearch(
	ctx context.Context,
	query string,
	scope domain.Scope,
	topK int,
) ([]domain.ScoredChunk, error) {
	vecs, err := s.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("search.KeywordSearch embed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("search.KeywordSearch: embedder returned no vectors for one input")
	}
	chunks, err := s.VectorStore.Search(ctx, vecs[0], scope, topK)
	if err != nil {
		return nil, fmt.Errorf("search.KeywordSearch store: %w", err)
	}
	return chunks, nil
}

// RAGSearch performs semantic search then filters results by minScore.
// It returns up to topK chunks with Score >= minScore, preserving the
// descending-score order returned by SemanticSearch. When no chunks meet the
// threshold an empty (non-nil) slice is returned — this is not an error.
// Input validation (topK default/max, minScore range) is enforced at the
// adapter boundary, not here. Implements FR-014.
func (s *SearchService) RAGSearch(
	ctx context.Context,
	query string,
	scope domain.Scope,
	topK int,
	minScore float32,
) ([]domain.ScoredChunk, error) {
	all, err := s.SemanticSearch(ctx, query, scope, topK)
	if err != nil {
		return nil, fmt.Errorf("search.RAGSearch: %w", err)
	}
	kept := make([]domain.ScoredChunk, 0, len(all))
	for _, c := range all {
		if c.Score >= minScore {
			kept = append(kept, c)
		}
	}
	return kept, nil
}
