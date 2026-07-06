// Package mcpadapter contains the MCP tool registrations and handler
// functions for the tahu daemon. Handlers are thin: they validate adapter-
// boundary inputs then delegate to the use-case layer.
package mcpadapter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// Size limits enforced at the adapter boundary (FR-020).
const (
	MaxBodyBytes   = 1 << 20 // 1 MB — concept body
	MaxStringBytes = 4096    // 4 KB — all other string inputs
)

// ParseScope delegates to domain.ParseScope. It is exposed here so tools.go
// can call it without importing the domain package directly and to provide a
// single validation entry point at the adapter boundary.
func ParseScope(s string) (domain.Scope, error) {
	return domain.ParseScope(s)
}

// ValidatePath is a fast pre-filter that rejects raw ".." components before
// the request reaches the repository boundary. The repository's
// BundlePathResolver enforces full containment and symlink checks.
// This pre-filter is defense-in-depth at the adapter boundary.
//
// Rationale: filepath.Clean normalises traversal in both directions. Checking
// before Clean means we refuse the raw input even if Clean would make it safe.
// This is the defensive check required by FR-019.
func ValidatePath(bundleAlias, relPath string) error {
	// Iterate both "/" and the platform separator so the check works on
	// Windows paths too (though NG8 defers Windows support).
	for _, sep := range []string{"/", string(filepath.Separator)} {
		for _, part := range strings.Split(relPath, sep) {
			if part == ".." {
				return fmt.Errorf("bundle %q path %q: %w",
					bundleAlias, relPath, domain.ErrPathEscape)
			}
		}
	}
	return nil
}

// ValidateInputSize returns domain.ErrInputTooLarge if the byte length of
// input exceeds maxBytes. This is called at the adapter boundary before the
// use-case layer is entered (FR-020).
func ValidateInputSize(input string, maxBytes int) error {
	if len(input) > maxBytes {
		return fmt.Errorf("%w: input is %d bytes, limit is %d",
			domain.ErrInputTooLarge, len(input), maxBytes)
	}
	return nil
}
