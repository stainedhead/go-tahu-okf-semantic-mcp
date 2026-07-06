package okf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// BundlePathResolver is the single gateway for converting (bundleAlias, relPath)
// into a validated absolute path. All filesystem access in FileNodeRepository
// goes through this resolver.
type BundlePathResolver struct {
	roots map[string]string // alias → absolute bundle root
}

// NewBundlePathResolver constructs a resolver from an alias→root map.
// Root paths must be absolute.
func NewBundlePathResolver(roots map[string]string) *BundlePathResolver {
	r := make(map[string]string, len(roots))
	for k, v := range roots {
		r[k] = v
	}
	return &BundlePathResolver{roots: r}
}

// bundleRoot returns the configured root for alias, or ErrNotFound.
func (r *BundlePathResolver) bundleRoot(alias string) (string, error) {
	root, ok := r.roots[alias]
	if !ok {
		return "", fmt.Errorf("bundle %q: %w", alias, domain.ErrNotFound)
	}
	return root, nil
}

// canonicalRoot resolves the bundle root's symlinks to get its real path.
func (r *BundlePathResolver) canonicalRoot(alias string) (string, error) {
	root, err := r.bundleRoot(alias)
	if err != nil {
		return "", err
	}
	canon, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("bundle %q: resolve root: %w", alias, err)
	}
	return canon, nil
}

// containedPath returns the cleaned joined path and verifies it is strictly
// below canonicalRoot. Returns ErrPathEscape if containment fails.
func containedPath(canonicalRoot, relPath string) (string, error) {
	cleaned := filepath.Clean(filepath.Join(canonicalRoot, relPath))
	rootWithSep := canonicalRoot
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}
	if !strings.HasPrefix(cleaned, rootWithSep) {
		return "", fmt.Errorf("%w: %q", domain.ErrPathEscape, relPath)
	}
	return cleaned, nil
}

// symGuardExisting checks the resolved absolute path for symlink escapes.
// Use for paths that already exist on disk.
func symGuardExisting(absPath, canonicalRoot string) error {
	real, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist, skip the symlink check.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("symlink resolve %q: %w", absPath, err)
	}
	rootWithSep := canonicalRoot
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}
	if !strings.HasPrefix(real+string(filepath.Separator), rootWithSep) &&
		real != strings.TrimSuffix(canonicalRoot, string(filepath.Separator)) {
		return fmt.Errorf("%w: symlink %q escapes bundle root", domain.ErrPathEscape, absPath)
	}
	return nil
}

// symGuardNew checks the final path component for symlinks without requiring
// the path to exist. Use for new concept writes.
// Per spec: if Lstat returns no error and entry is a symlink → reject.
// If Lstat returns ErrNotExist → path is new and safe.
func symGuardNew(absPath string) error {
	info, err := os.Lstat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // new path, safe
		}
		return fmt.Errorf("lstat %q: %w", absPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %q is a symlink", domain.ErrPathEscape, absPath)
	}
	return nil
}

// Resolve validates and resolves (alias, relPath) for reading existing concept files.
// Rejects traversal, reserved names, and symlink escapes.
func (r *BundlePathResolver) Resolve(alias, relPath string) (string, error) {
	canon, err := r.canonicalRoot(alias)
	if err != nil {
		return "", err
	}
	absPath, err := containedPath(canon, relPath)
	if err != nil {
		return "", err
	}
	// Reject reserved filenames.
	base := filepath.Base(absPath)
	if base == "index.md" || base == "log.md" {
		return "", fmt.Errorf("%w: %q", domain.ErrReservedPath, base)
	}
	if err := symGuardExisting(absPath, canon); err != nil {
		return "", err
	}
	return absPath, nil
}

// ResolveNew validates and resolves (alias, relPath) for writing new concept files.
// Uses Lstat instead of EvalSymlinks for the final component since the target may not exist.
func (r *BundlePathResolver) ResolveNew(alias, relPath string) (string, error) {
	canon, err := r.canonicalRoot(alias)
	if err != nil {
		return "", err
	}
	absPath, err := containedPath(canon, relPath)
	if err != nil {
		return "", err
	}
	// Reject reserved filenames.
	base := filepath.Base(absPath)
	if base == "index.md" || base == "log.md" {
		return "", fmt.Errorf("%w: %q", domain.ErrReservedPath, base)
	}
	if err := symGuardNew(absPath); err != nil {
		return "", err
	}
	return absPath, nil
}

// ResolveReserved validates and resolves (alias, relPath) for reserved files
// (index.md, log.md). These bypass the reserved-name rejection but still enforce
// containment and symlink guards.
func (r *BundlePathResolver) ResolveReserved(alias, relPath string) (string, error) {
	canon, err := r.canonicalRoot(alias)
	if err != nil {
		return "", err
	}
	absPath, err := containedPath(canon, relPath)
	if err != nil {
		return "", err
	}
	if err := symGuardNew(absPath); err != nil {
		return "", err
	}
	return absPath, nil
}

// BundleRoot returns the raw (unresolved) root path for alias.
func (r *BundlePathResolver) BundleRoot(alias string) (string, error) {
	return r.bundleRoot(alias)
}
