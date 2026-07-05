package okf

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// ValidateConceptPath ensures that relPath, when joined with bundleRoot, stays
// within the bundle root and does not target a reserved filename.
//
// Security model:
//   - The bundle root is canonicalized via filepath.EvalSymlinks so that the
//     containment check compares against the real path even on systems where
//     /tmp → /private/tmp (macOS).
//   - The joined path is cleaned with filepath.Clean before the prefix check,
//     which resolves any ".." traversal sequences in relPath regardless of
//     whether the target file exists yet — making it safe to call for new
//     concepts that have not been written.
func ValidateConceptPath(bundleRoot, relPath string) error {
	canonicalRoot, err := filepath.EvalSymlinks(bundleRoot)
	if err != nil {
		return fmt.Errorf("ValidateConceptPath: resolve bundle root: %w", err)
	}

	// Join the symlink-resolved root with the caller-supplied path and clean
	// the result. filepath.Clean resolves ".." sequences without touching the
	// filesystem, so this is safe even for non-existent targets.
	cleaned := filepath.Clean(filepath.Join(canonicalRoot, relPath))

	// The cleaned path must start with canonicalRoot + separator, ensuring it
	// is strictly below the root (not the root directory itself).
	rootWithSep := canonicalRoot
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}

	if !strings.HasPrefix(cleaned, rootWithSep) {
		return fmt.Errorf("%w: %q", domain.ErrPathEscape, relPath)
	}

	// Reject reserved filenames at any directory level.
	base := filepath.Base(cleaned)
	if base == "index.md" || base == "log.md" {
		return fmt.Errorf("%w: %q", domain.ErrReservedPath, base)
	}

	return nil
}

// ValidateFrontmatter checks that the required OKF frontmatter fields are
// present.
func ValidateFrontmatter(fm domain.OKFFrontmatter) error {
	if fm.Type == "" {
		return domain.ErrMissingType
	}
	return nil
}
