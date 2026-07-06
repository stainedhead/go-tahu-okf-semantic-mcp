// Package usecase contains the application-level orchestration logic for the
// tahu daemon. It depends only on domain types and interfaces; it never imports
// adapter or infra packages.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// BundleService implements bundle management use cases FR-001 through FR-004.
type BundleService struct {
	BundleRepository domain.BundleRepository
	NodeRepository   domain.NodeRepository
	Now              func() time.Time // injected clock; defaults to time.Now
}

// AddBundle registers a new OKF bundle at rootPath with the given alias (FR-001).
//
// Invariants enforced:
//   - rootPath must exist on disk (os.Stat).
//   - rootPath must contain at least one .md file (filepath.WalkDir).
//   - alias must be unique across all registered bundles (ErrDuplicateAlias).
//   - rootPath must not already be registered under a different alias (ErrDuplicatePath).
func (s *BundleService) AddBundle(
	ctx context.Context,
	alias, rootPath, description string,
	tags []string,
) (*domain.BundleEntry, error) {
	// 1. Validate rootPath exists on disk.
	if _, err := os.Stat(rootPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("AddBundle %q: rootPath does not exist: %w", alias, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("AddBundle %q: stat rootPath: %w", alias, err)
	}

	// 2. Validate at least one .md file exists under rootPath.
	hasMD := false
	if walkErr := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".md" {
			hasMD = true
			return fs.SkipAll
		}
		return nil
	}); walkErr != nil {
		return nil, fmt.Errorf("AddBundle %q: walk rootPath: %w", alias, walkErr)
	}
	if !hasMD {
		return nil, fmt.Errorf("AddBundle %q: rootPath contains no .md files: %w", alias, domain.ErrNotFound)
	}

	// 3. Reject duplicate alias.
	existing, err := s.BundleRepository.Get(ctx, alias)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("AddBundle %q: lookup alias: %w", alias, err)
	}
	if existing != nil {
		return nil, fmt.Errorf("AddBundle %q: %w", alias, domain.ErrDuplicateAlias)
	}

	// 4. Reject duplicate rootPath registered under any other alias.
	all, err := s.BundleRepository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("AddBundle %q: list bundles: %w", alias, err)
	}
	for _, b := range all {
		if b.RootPath == rootPath {
			return nil, fmt.Errorf(
				"AddBundle %q: rootPath %q is already registered as alias %q: %w",
				alias, rootPath, b.Alias, domain.ErrDuplicatePath,
			)
		}
	}

	// 5. Persist.
	now := s.now()
	entry := domain.BundleEntry{
		Alias:       alias,
		RootPath:    rootPath,
		Description: description,
		Tags:        tags,
		CreatedAt:   now,
	}
	if putErr := s.BundleRepository.Put(ctx, entry); putErr != nil {
		return nil, fmt.Errorf("AddBundle %q: persist: %w", alias, putErr)
	}
	return &entry, nil
}

// RemoveBundle unregisters a bundle by alias without deleting any files (FR-002).
// Returns a wrapped ErrNotFound if the alias is not registered.
func (s *BundleService) RemoveBundle(ctx context.Context, alias string) error {
	if _, err := s.BundleRepository.Get(ctx, alias); err != nil {
		return fmt.Errorf("RemoveBundle %q: %w", alias, err)
	}
	if err := s.BundleRepository.Delete(ctx, alias); err != nil {
		return fmt.Errorf("RemoveBundle %q: delete: %w", alias, err)
	}
	return nil
}

// ListBundles returns all registered bundles (FR-003).
// The ConceptCount field of each BundleEntry is populated by calling
// NodeRepository.List. If NodeRepository.List fails for an individual bundle,
// ConceptCount stays zero and the overall list still succeeds.
func (s *BundleService) ListBundles(ctx context.Context) ([]domain.BundleEntry, error) {
	bundles, err := s.BundleRepository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListBundles: %w", err)
	}
	for i := range bundles {
		refs, listErr := s.NodeRepository.List(ctx, bundles[i].Alias, "")
		if listErr == nil {
			bundles[i].ConceptCount = len(refs)
		}
	}
	return bundles, nil
}

// ReindexBundle forces a full re-embed and reindex of a bundle (FR-004).
// It retrieves every concept in the bundle, embeds each body as a single chunk,
// upserts all chunks to the vector store, and updates LastIndexedAt.
func (s *BundleService) ReindexBundle(
	ctx context.Context,
	alias string,
	embedder domain.Embedder,
	store domain.VectorStore,
) error {
	// 1. Resolve bundle entry — fail fast with ErrNotFound if alias unknown.
	entry, err := s.BundleRepository.Get(ctx, alias)
	if err != nil {
		return fmt.Errorf("ReindexBundle %q: get bundle: %w", alias, err)
	}

	// 2. List all concept refs in the bundle.
	refs, err := s.NodeRepository.List(ctx, alias, "")
	if err != nil {
		return fmt.Errorf("ReindexBundle %q: list concepts: %w", alias, err)
	}

	// 3. Retrieve every concept to obtain body text for embedding.
	chunks := make([]domain.EmbeddingChunk, 0, len(refs))
	texts := make([]string, 0, len(refs))
	newChunkIDs := make([]string, 0, len(refs))
	for _, ref := range refs {
		concept, getErr := s.NodeRepository.Get(ctx, ref)
		if getErr != nil {
			return fmt.Errorf("ReindexBundle %q: get concept %s: %w", alias, ref, getErr)
		}
		chunkID := fmt.Sprintf("%s:%s:0", alias, ref.RelativePath)
		chunks = append(chunks, domain.EmbeddingChunk{
			ID:                 chunkID,
			BundleAlias:        alias,
			ConceptPath:        ref.RelativePath,
			ChunkIndex:         0,
			Text:               concept.Body,
			FrontmatterSummary: concept.Frontmatter.Type + ":" + concept.Frontmatter.Title,
		})
		texts = append(texts, concept.Body)
		newChunkIDs = append(newChunkIDs, chunkID)
	}

	// 4. Delete all previously indexed chunks for this bundle.  This is done
	// unconditionally (even when refs is empty) so that deleting the last
	// concept in a bundle cleans up its stale vector-store entries.
	if deleteErr := store.Delete(ctx, entry.ChunkIDs); deleteErr != nil {
		return fmt.Errorf("ReindexBundle %q: delete stale chunks: %w", alias, deleteErr)
	}

	// 5. Embed and upsert — skip the network round-trip if there is nothing to index.
	if len(texts) > 0 {
		embeddings, embedErr := embedder.Embed(ctx, texts)
		if embedErr != nil {
			return fmt.Errorf("ReindexBundle %q: embed: %w", alias, embedErr)
		}
		for i := range chunks {
			chunks[i].Embedding = embeddings[i]
		}
		if upsertErr := store.Upsert(ctx, chunks); upsertErr != nil {
			return fmt.Errorf("ReindexBundle %q: upsert to vector store: %w", alias, upsertErr)
		}
	}

	// 6. Stamp LastIndexedAt, record new chunk IDs, and persist.
	entry.LastIndexedAt = s.now()
	entry.ChunkIDs = newChunkIDs
	if putErr := s.BundleRepository.Put(ctx, *entry); putErr != nil {
		return fmt.Errorf("ReindexBundle %q: update bundle entry: %w", alias, putErr)
	}
	return nil
}

// now returns the current time using the injected clock or time.Now.
func (s *BundleService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
