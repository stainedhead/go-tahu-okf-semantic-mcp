// Package registry provides a YAML-backed implementation of domain.BundleRepository.
// It persists bundle registration metadata to a single YAML file and is safe
// for concurrent use via sync.RWMutex.
package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// Compile-time assertion.
var _ domain.BundleRepository = (*YAMLBundleRepository)(nil)

// yamlEntry is the serialised form of a BundleEntry. ConceptCount is excluded
// because it is derived at runtime by the use-case layer.
type yamlEntry struct {
	Alias         string    `yaml:"alias"`
	RootPath      string    `yaml:"root_path"`
	Description   string    `yaml:"description,omitempty"`
	Tags          []string  `yaml:"tags,omitempty"`
	CreatedAt     time.Time `yaml:"created_at"`
	LastIndexedAt time.Time `yaml:"last_indexed_at,omitempty"`
	ChunkIDs      []string  `yaml:"chunk_ids,omitempty"`
}

// yamlFile is the top-level document written to disk.
type yamlFile struct {
	Bundles []yamlEntry `yaml:"bundles"`
}

// YAMLBundleRepository implements domain.BundleRepository backed by a YAML
// file. Each mutating operation loads then saves the file atomically. This
// approach is simple and correct for registries of up to a few hundred bundles.
type YAMLBundleRepository struct {
	mu       sync.RWMutex
	path     string
	lockPath string
}

// NewYAMLBundleRepository creates a YAMLBundleRepository that persists to
// path. The file and its parent directories are created on first write.
func NewYAMLBundleRepository(path string) *YAMLBundleRepository {
	return &YAMLBundleRepository{
		path:     path,
		lockPath: path + ".lock",
	}
}

// Get retrieves a bundle by alias. Returns a wrapped domain.ErrNotFound when
// the alias is not registered.
func (r *YAMLBundleRepository) Get(_ context.Context, alias string) (*domain.BundleEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, e := range reg.Bundles {
		if e.Alias == alias {
			entry := fromYAML(e)
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("bundle %q: %w", alias, domain.ErrNotFound)
}

// Put creates or replaces the entry for entry.Alias.
func (r *YAMLBundleRepository) Put(_ context.Context, entry domain.BundleEntry) error {
	fl, err := newFileLock(r.lockPath)
	if err != nil {
		return err
	}
	defer fl.close() //nolint:errcheck
	if err := fl.lock(); err != nil {
		return fmt.Errorf("registry.Put: acquire lock: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	reg, err := r.load()
	if err != nil {
		return err
	}
	y := toYAML(entry)
	for i, e := range reg.Bundles {
		if e.Alias == entry.Alias {
			reg.Bundles[i] = y
			return r.save(reg)
		}
	}
	reg.Bundles = append(reg.Bundles, y)
	return r.save(reg)
}

// Delete removes the entry for alias. No-op if alias is not registered.
func (r *YAMLBundleRepository) Delete(_ context.Context, alias string) error {
	fl, err := newFileLock(r.lockPath)
	if err != nil {
		return err
	}
	defer fl.close() //nolint:errcheck
	if err := fl.lock(); err != nil {
		return fmt.Errorf("registry.Delete: acquire lock: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	reg, err := r.load()
	if err != nil {
		return err
	}
	filtered := reg.Bundles[:0]
	for _, e := range reg.Bundles {
		if e.Alias != alias {
			filtered = append(filtered, e)
		}
	}
	reg.Bundles = filtered
	return r.save(reg)
}

// List returns all registered bundles in insertion order.
func (r *YAMLBundleRepository) List(_ context.Context) ([]domain.BundleEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, err := r.load()
	if err != nil {
		return nil, err
	}
	entries := make([]domain.BundleEntry, len(reg.Bundles))
	for i, e := range reg.Bundles {
		entries[i] = fromYAML(e)
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// load reads the YAML file from disk. Returns an empty registry if the file
// does not exist.
func (r *YAMLBundleRepository) load() (*yamlFile, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &yamlFile{}, nil
		}
		return nil, fmt.Errorf("registry.load %s: %w", r.path, err)
	}
	var f yamlFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("registry.load %s: unmarshal: %w", r.path, err)
	}
	return &f, nil
}

// save atomically writes f to disk via a temp-file + rename.
func (r *YAMLBundleRepository) save(f *yamlFile) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("registry.save: marshal: %w", err)
	}
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // dir is from config-supplied registry path
		return fmt.Errorf("registry.save: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tahu-registry-*.tmp")
	if err != nil {
		return fmt.Errorf("registry.save: create temp: %w", err)
	}
	tmpName := tmp.Name()
	var renamed bool
	defer func() {
		_ = tmp.Close()
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("registry.save: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("registry.save: close temp: %w", err)
	}
	if err := os.Rename(tmpName, r.path); err != nil {
		return fmt.Errorf("registry.save: rename: %w", err)
	}
	renamed = true
	return nil
}

func toYAML(e domain.BundleEntry) yamlEntry {
	return yamlEntry{
		Alias:         e.Alias,
		RootPath:      e.RootPath,
		Description:   e.Description,
		Tags:          e.Tags,
		CreatedAt:     e.CreatedAt,
		LastIndexedAt: e.LastIndexedAt,
		ChunkIDs:      e.ChunkIDs,
	}
}

func fromYAML(y yamlEntry) domain.BundleEntry {
	return domain.BundleEntry{
		Alias:         y.Alias,
		RootPath:      y.RootPath,
		Description:   y.Description,
		Tags:          y.Tags,
		CreatedAt:     y.CreatedAt,
		LastIndexedAt: y.LastIndexedAt,
		ChunkIDs:      y.ChunkIDs,
	}
}
