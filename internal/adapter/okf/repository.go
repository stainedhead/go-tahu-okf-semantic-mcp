package okf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// Compile-time assertion that FileNodeRepository satisfies domain.NodeRepository.
var _ domain.NodeRepository = (*FileNodeRepository)(nil)

// maxFileBytes is the read cap for concept and reserved files (1 MB, matching
// the MCP write path MaxBodyBytes). Files larger than this are rejected before
// reading to prevent memory exhaustion.
const maxFileBytes = 1 << 20

// checkFileSize returns ErrInputTooLarge if the file at path exceeds maxFileBytes.
// It does not return an error if the file does not exist — the caller handles that.
func checkFileSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil // let the caller's ReadFile handle ErrNotExist
	}
	if info.Size() > maxFileBytes {
		return fmt.Errorf("%w: file is %d bytes (limit %d)", domain.ErrInputTooLarge, info.Size(), maxFileBytes)
	}
	return nil
}

// FileNodeRepository implements domain.NodeRepository backed by a filesystem
// of OKF markdown documents.
type FileNodeRepository struct {
	// resolver is the single gateway for all path resolution and validation.
	// All filesystem access goes through it, enforcing containment, reserved-name,
	// and symlink-escape checks.
	resolver *BundlePathResolver

	// mu serializes writes across all bundles.  A per-bundle mutex would be
	// more granular; a single mutex is simpler and correct for v0.1.
	mu sync.Mutex
}

// NewFileNodeRepository creates a FileNodeRepository with the given alias→root
// mapping.  root paths must be absolute and must already exist on disk.
func NewFileNodeRepository(roots map[string]string) *FileNodeRepository {
	return &FileNodeRepository{
		resolver: NewBundlePathResolver(roots),
	}
}

// Get retrieves a parsed OKFConcept by ref.
func (f *FileNodeRepository) Get(_ context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	absPath, err := f.resolver.Resolve(ref.BundleAlias, ref.RelativePath)
	if err != nil {
		return nil, err
	}

	if err := checkFileSize(absPath); err != nil {
		return nil, fmt.Errorf("concept %s: %w", ref, err)
	}

	data, err := os.ReadFile(absPath) //nolint:gosec // path validated by BundlePathResolver
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("concept %s: %w", ref, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("concept %s: read: %w", ref, err)
	}

	concept, err := ParseConcept(absPath, data)
	if err != nil {
		return nil, fmt.Errorf("concept %s: parse: %w", ref, err)
	}
	concept.Ref = ref

	root, err := f.resolver.BundleRoot(ref.BundleAlias)
	if err != nil {
		return nil, err
	}
	conceptDir := filepath.Dir(absPath)
	links, err := ExtractLinks(concept.Body, root, conceptDir)
	if err != nil {
		return nil, fmt.Errorf("concept %s: extract links: %w", ref, err)
	}
	concept.OutboundLinks = links

	return concept, nil
}

// Put creates or replaces a concept document on disk.  It does not regenerate
// index.md or append to log.md — that responsibility belongs to the use-case
// layer (ConceptService) which calls Put and then handles index/log updates.
func (f *FileNodeRepository) Put(_ context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	absPath, err := f.resolver.ResolveNew(ref.BundleAlias, ref.RelativePath)
	if err != nil {
		return err
	}

	if err := ValidateFrontmatter(concept.Frontmatter); err != nil {
		return err
	}

	data, err := SerializeConcept(concept)
	if err != nil {
		return fmt.Errorf("Put %s: serialize: %w", ref, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil { //nolint:gosec // path validated by BundlePathResolver
		return fmt.Errorf("Put %s: mkdir: %w", ref, err)
	}

	if err := os.WriteFile(absPath, data, 0o644); err != nil { //nolint:gosec // path validated by BundlePathResolver
		return fmt.Errorf("Put %s: write: %w", ref, err)
	}

	return nil
}

// List returns refs for all non-reserved .md files under subPath within the
// bundle.  If subPath is empty, the bundle root is searched.  A non-existent
// subPath returns an empty list rather than an error.
func (f *FileNodeRepository) List(_ context.Context, bundleAlias string, subPath string) ([]domain.ConceptRef, error) {
	root, err := f.resolver.BundleRoot(bundleAlias)
	if err != nil {
		return nil, err
	}

	// Validate subPath containment before using it as a walk root.
	if subPath != "" {
		if _, err := f.resolver.ResolveReserved(bundleAlias, subPath); err != nil {
			return nil, err
		}
	}

	searchRoot := root
	if subPath != "" {
		searchRoot = filepath.Join(root, subPath)
	}

	var refs []domain.ConceptRef
	err = filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		if d.Name() == "index.md" || d.Name() == "log.md" {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		refs = append(refs, domain.ConceptRef{
			BundleAlias:  bundleAlias,
			RelativePath: relPath,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("List %s/%s: walk: %w", bundleAlias, subPath, err)
	}

	return refs, nil
}

// ListTypes returns all distinct frontmatter type values in the bundle.
func (f *FileNodeRepository) ListTypes(ctx context.Context, bundleAlias string) ([]string, error) {
	refs, err := f.List(ctx, bundleAlias, "")
	if err != nil {
		return nil, err
	}

	root, err := f.resolver.BundleRoot(bundleAlias)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var types []string

	for _, ref := range refs {
		absPath := filepath.Join(root, ref.RelativePath)
		data, err := os.ReadFile(absPath) //nolint:gosec // path comes from List which walks the validated root
		if err != nil {
			continue
		}
		concept, err := ParseConcept(absPath, data)
		if err != nil {
			continue
		}
		t := concept.Frontmatter.Type
		if t != "" && !seen[t] {
			seen[t] = true
			types = append(types, t)
		}
	}

	return types, nil
}

// ReadReserved returns the raw content of a reserved file at relPath within
// the bundle.  Returns domain.ErrNotFound if the file does not exist.
func (f *FileNodeRepository) ReadReserved(_ context.Context, bundleAlias string, relPath string) (string, error) {
	absPath, err := f.resolver.ResolveReserved(bundleAlias, relPath)
	if err != nil {
		return "", err
	}

	if err := checkFileSize(absPath); err != nil {
		return "", fmt.Errorf("ReadReserved %s/%s: %w", bundleAlias, relPath, err)
	}

	data, err := os.ReadFile(absPath) //nolint:gosec // path validated by BundlePathResolver
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("reserved file %s/%s: %w", bundleAlias, relPath, domain.ErrNotFound)
		}
		return "", fmt.Errorf("ReadReserved %s/%s: %w", bundleAlias, relPath, err)
	}

	return string(data), nil
}

// WriteReserved creates or replaces a reserved file at relPath within the bundle.
func (f *FileNodeRepository) WriteReserved(_ context.Context, bundleAlias string, relPath string, content string) error {
	absPath, err := f.resolver.ResolveReserved(bundleAlias, relPath)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil { //nolint:gosec // path validated by BundlePathResolver
		return fmt.Errorf("WriteReserved %s/%s: mkdir: %w", bundleAlias, relPath, err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil { //nolint:gosec // path validated by BundlePathResolver
		return fmt.Errorf("WriteReserved %s/%s: write: %w", bundleAlias, relPath, err)
	}

	return nil
}
