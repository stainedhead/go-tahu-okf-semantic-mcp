package usecase

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// ConceptService implements the OKF read/write/navigate use cases (FR-005
// through FR-011). It depends only on domain interfaces; no adapter or infra
// packages are imported.
type ConceptService struct {
	NodeRepository   domain.NodeRepository
	BundleRepository domain.BundleRepository
	bundleMu         sync.Map // maps bundle alias (string) -> *sync.Mutex
}

// bundleLock returns the per-bundle advisory mutex, creating it on first use.
// Serializes the full WriteConcept flow (Put + regenerateIndex + appendLog)
// per bundle so concurrent writes cannot interleave their read-modify-write
// on log.md (FIX-004).
func (s *ConceptService) bundleLock(alias string) *sync.Mutex {
	v, _ := s.bundleMu.LoadOrStore(alias, &sync.Mutex{})
	return v.(*sync.Mutex) //nolint:forcetypeassert
}

// ReadConcept returns the parsed OKF concept for ref (FR-005).
// Returns a wrapped domain.ErrNotFound when the concept does not exist.
func (s *ConceptService) ReadConcept(ctx context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	concept, err := s.NodeRepository.Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("concept_read %s: %w", ref, err)
	}
	return concept, nil
}

// ListConcepts returns refs for all non-reserved .md files at subPath within
// bundleAlias (FR-006). An empty subPath lists the bundle root. Returns an
// empty (non-nil) slice when the directory does not exist or contains no
// concepts — never an error in that case.
func (s *ConceptService) ListConcepts(ctx context.Context, bundleAlias, subPath string) ([]domain.ConceptRef, error) {
	refs, err := s.NodeRepository.List(ctx, bundleAlias, subPath)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return []domain.ConceptRef{}, nil
		}
		return nil, fmt.Errorf("concept_list %s: %w", bundleAlias, err)
	}
	if refs == nil {
		return []domain.ConceptRef{}, nil
	}
	return refs, nil
}

// GetLinks returns all outbound markdown hyperlinks found in the concept body
// (FR-007). Broken links are included with Broken set to true.
func (s *ConceptService) GetLinks(ctx context.Context, ref domain.ConceptRef) ([]domain.ConceptLink, error) {
	concept, err := s.NodeRepository.Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("concept_links %s: %w", ref, err)
	}
	return concept.OutboundLinks, nil
}

// ReadIndex returns the raw content of index.md at dirPath within bundleAlias
// (FR-008). dirPath may be empty to address the bundle root. Returns a
// wrapped domain.ErrNotFound when the file is absent.
func (s *ConceptService) ReadIndex(ctx context.Context, bundleAlias, dirPath string) (string, error) {
	relPath := reservedPath(dirPath, "index.md")
	content, err := s.NodeRepository.ReadReserved(ctx, bundleAlias, relPath)
	if err != nil {
		return "", fmt.Errorf("index_read %s:%s: %w", bundleAlias, relPath, err)
	}
	return content, nil
}

// ReadLog returns the raw content of log.md at dirPath within bundleAlias
// (FR-009). dirPath may be empty to address the bundle root. Returns a
// wrapped domain.ErrNotFound when the file is absent.
func (s *ConceptService) ReadLog(ctx context.Context, bundleAlias, dirPath string) (string, error) {
	relPath := reservedPath(dirPath, "log.md")
	content, err := s.NodeRepository.ReadReserved(ctx, bundleAlias, relPath)
	if err != nil {
		return "", fmt.Errorf("log_read %s:%s: %w", bundleAlias, relPath, err)
	}
	return content, nil
}

// ListTypes returns all distinct frontmatter type values present in
// bundleAlias (FR-010).
func (s *ConceptService) ListTypes(ctx context.Context, bundleAlias string) ([]string, error) {
	types, err := s.NodeRepository.ListTypes(ctx, bundleAlias)
	if err != nil {
		return nil, fmt.Errorf("concept_type_list %s: %w", bundleAlias, err)
	}
	return types, nil
}

// WriteConcept creates or updates an OKF concept document (FR-011).
//
// Preconditions validated before any write:
//   - concept.Frontmatter.Type must be non-empty (ErrMissingType)
//   - path.Base(ref.RelativePath) must not be "index.md" or "log.md" (ErrReservedPath)
//
// On success the method:
//  1. Persists the concept via NodeRepository.Put.
//  2. Regenerates index.md for the directory containing ref.
//  3. Appends a timestamped entry to log.md in the same directory (creating
//     log.md if it does not exist).
func (s *ConceptService) WriteConcept(ctx context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	// Validate: type field is required.
	if strings.TrimSpace(concept.Frontmatter.Type) == "" {
		return fmt.Errorf("concept_write %s: %w", ref, domain.ErrMissingType)
	}

	// Validate: target filename must not be a reserved name at any level.
	base := path.Base(ref.RelativePath)
	if base == "index.md" || base == "log.md" {
		return fmt.Errorf("concept_write %s: %w", ref, domain.ErrReservedPath)
	}

	// Defense-in-depth: validate the ref at the use-case layer before handing off
	// to the repository. The repository enforces full containment, but an explicit
	// check here makes the boundary visible in code review.
	if ref.RelativePath == "" {
		return fmt.Errorf("WriteConcept: %w: empty relative path", domain.ErrPathEscape)
	}
	for _, part := range strings.Split(ref.RelativePath, "/") {
		if part == ".." {
			return fmt.Errorf("WriteConcept: %w: traversal in path", domain.ErrPathEscape)
		}
	}

	// Serialize the full write sequence per bundle to prevent the
	// ReadReserved→WriteReserved in appendLog from racing with concurrent
	// WriteConcept calls (FIX-004).
	mu := s.bundleLock(ref.BundleAlias)
	mu.Lock()
	defer mu.Unlock()

	// Persist the concept.
	if err := s.NodeRepository.Put(ctx, ref, concept); err != nil {
		return fmt.Errorf("concept_write %s: %w", ref, err)
	}

	// Regenerate index.md for the containing directory.
	if err := s.regenerateIndex(ctx, ref); err != nil {
		return fmt.Errorf("concept_write %s: regenerate index: %w", ref, err)
	}

	// Append timestamped entry to log.md (creates it if absent).
	if err := s.appendLog(ctx, ref); err != nil {
		return fmt.Errorf("concept_write %s: append log: %w", ref, err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// regenerateIndex lists all concepts in ref's directory and writes a fresh
// index.md with File, Type, and Title columns populated from frontmatter.
// A List result of ErrNotFound is treated as an empty directory. Get failures
// for individual refs are tolerated — they produce empty type/title cells.
func (s *ConceptService) regenerateIndex(ctx context.Context, ref domain.ConceptRef) error {
	dir := conceptDir(ref.RelativePath)
	refs, err := s.NodeRepository.List(ctx, ref.BundleAlias, dir)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# Index\n\n")
	if len(refs) > 0 {
		sb.WriteString("| File | Type | Title |\n|---|---|---|\n")
		for _, r := range refs {
			name := path.Base(r.RelativePath)
			typ, title := "", ""
			if c, err := s.NodeRepository.Get(ctx, r); err == nil {
				typ = c.Frontmatter.Type
				title = c.Frontmatter.Title
			}
			fmt.Fprintf(&sb, "| [%s](%s) | %s | %s |\n", name, name, typ, title)
		}
	}

	return s.NodeRepository.WriteReserved(ctx, ref.BundleAlias, reservedPath(dir, "index.md"), sb.String())
}

// appendLog appends a single timestamped write entry to log.md in ref's
// directory. If log.md does not yet exist it is created.
func (s *ConceptService) appendLog(ctx context.Context, ref domain.ConceptRef) error {
	dir := conceptDir(ref.RelativePath)
	logRelPath := reservedPath(dir, "log.md")

	existing, err := s.NodeRepository.ReadReserved(ctx, ref.BundleAlias, logRelPath)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	entry := fmt.Sprintf("- %s: concept_write `%s`\n",
		time.Now().UTC().Format(time.RFC3339), ref)

	return s.NodeRepository.WriteReserved(ctx, ref.BundleAlias, logRelPath, existing+entry)
}

// conceptDir returns the directory portion of a concept relative path,
// using "" for root-level concepts (avoiding the path.Dir "." sentinel).
func conceptDir(relPath string) string {
	d := path.Dir(relPath)
	if d == "." {
		return ""
	}
	return d
}

// reservedPath computes the relative path for a reserved file (index.md or
// log.md) within the given directory. An empty dir means the bundle root.
func reservedPath(dir, filename string) string {
	if dir == "" {
		return filename
	}
	return dir + "/" + filename
}
