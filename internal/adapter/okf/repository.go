package okf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// Compile-time assertion that FileNodeRepository satisfies domain.NodeRepository.
var _ domain.NodeRepository = (*FileNodeRepository)(nil)

// FileNodeRepository implements domain.NodeRepository backed by a filesystem
// of OKF markdown documents.
type FileNodeRepository struct {
	// roots maps bundle alias to absolute bundle root path (as supplied by the
	// caller; may or may not be symlink-resolved — ValidateConceptPath handles
	// canonicalization).
	roots map[string]string

	// mu serializes writes across all bundles.  A per-bundle mutex would be
	// more granular; a single mutex is simpler and correct for v0.1.
	mu sync.Mutex
}

// NewFileNodeRepository creates a FileNodeRepository with the given alias→root
// mapping.  root paths must be absolute and must already exist on disk.
func NewFileNodeRepository(roots map[string]string) *FileNodeRepository {
	r := make(map[string]string, len(roots))
	for k, v := range roots {
		r[k] = v
	}
	return &FileNodeRepository{roots: r}
}

// bundleRoot returns the root path for alias, or ErrNotFound.
func (f *FileNodeRepository) bundleRoot(alias string) (string, error) {
	root, ok := f.roots[alias]
	if !ok {
		return "", fmt.Errorf("bundle %q: %w", alias, domain.ErrNotFound)
	}
	return root, nil
}

// Get retrieves a parsed OKFConcept by ref.
func (f *FileNodeRepository) Get(ctx context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	root, err := f.bundleRoot(ref.BundleAlias)
	if err != nil {
		return nil, err
	}

	if err := ValidateConceptPath(root, ref.RelativePath); err != nil {
		return nil, err
	}

	absPath := filepath.Join(root, ref.RelativePath)
	data, err := os.ReadFile(absPath)
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

	conceptDir := filepath.Dir(absPath)
	links, err := ExtractLinks(concept.Body, root, conceptDir)
	if err != nil {
		return nil, fmt.Errorf("concept %s: extract links: %w", ref, err)
	}
	concept.OutboundLinks = links

	return concept, nil
}

// Put creates or replaces a concept document, then regenerates index.md and
// appends an entry to log.md for the affected directory.
func (f *FileNodeRepository) Put(ctx context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	root, err := f.bundleRoot(ref.BundleAlias)
	if err != nil {
		return err
	}

	if err := ValidateConceptPath(root, ref.RelativePath); err != nil {
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

	absPath := filepath.Join(root, ref.RelativePath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("Put %s: mkdir: %w", ref, err)
	}

	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return fmt.Errorf("Put %s: write: %w", ref, err)
	}

	// Regenerate index.md for the affected directory.
	dirPath := filepath.Dir(absPath)
	indexContent, err := GenerateIndex(root, dirPath)
	if err != nil {
		return fmt.Errorf("Put %s: generate index: %w", ref, err)
	}
	indexPath := filepath.Join(dirPath, "index.md")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0o644); err != nil {
		return fmt.Errorf("Put %s: write index: %w", ref, err)
	}

	// Append timestamped entry to log.md.
	logEntry := fmt.Sprintf("wrote %s", ref.RelativePath)
	if err := AppendLog(root, dirPath, logEntry, time.Now()); err != nil {
		return fmt.Errorf("Put %s: append log: %w", ref, err)
	}

	return nil
}

// List returns refs for all non-reserved .md files under subPath within the
// bundle.  If subPath is empty, the bundle root is searched.  A non-existent
// subPath returns an empty list rather than an error.
func (f *FileNodeRepository) List(ctx context.Context, bundleAlias string, subPath string) ([]domain.ConceptRef, error) {
	root, err := f.bundleRoot(bundleAlias)
	if err != nil {
		return nil, err
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

	root, err := f.bundleRoot(bundleAlias)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var types []string

	for _, ref := range refs {
		absPath := filepath.Join(root, ref.RelativePath)
		data, err := os.ReadFile(absPath)
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
func (f *FileNodeRepository) ReadReserved(ctx context.Context, bundleAlias string, relPath string) (string, error) {
	root, err := f.bundleRoot(bundleAlias)
	if err != nil {
		return "", err
	}

	absPath := filepath.Join(root, filepath.Clean(relPath))
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("reserved file %s/%s: %w", bundleAlias, relPath, domain.ErrNotFound)
		}
		return "", fmt.Errorf("ReadReserved %s/%s: %w", bundleAlias, relPath, err)
	}

	return string(data), nil
}

// WriteReserved creates or replaces a reserved file at relPath within the bundle.
func (f *FileNodeRepository) WriteReserved(ctx context.Context, bundleAlias string, relPath string, content string) error {
	root, err := f.bundleRoot(bundleAlias)
	if err != nil {
		return err
	}

	absPath := filepath.Join(root, filepath.Clean(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("WriteReserved %s/%s: mkdir: %w", bundleAlias, relPath, err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("WriteReserved %s/%s: write: %w", bundleAlias, relPath, err)
	}

	return nil
}
