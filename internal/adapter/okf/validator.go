package okf

import (
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// ValidateFrontmatter checks that the required OKF frontmatter fields are
// present.
func ValidateFrontmatter(fm domain.OKFFrontmatter) error {
	if fm.Type == "" {
		return domain.ErrMissingType
	}
	return nil
}
