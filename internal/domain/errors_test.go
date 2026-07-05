package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// TestSentinelErrors_ErrorsIs verifies that every sentinel domain error works
// correctly with errors.Is, including when wrapped via fmt.Errorf("%w", ...).
// Each sentinel must match itself and its wrapped form, and must NOT match any
// other sentinel.
func TestSentinelErrors_ErrorsIs(t *testing.T) {
	t.Parallel()

	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", domain.ErrNotFound},
		{"ErrReservedPath", domain.ErrReservedPath},
		{"ErrMissingType", domain.ErrMissingType},
		{"ErrPathEscape", domain.ErrPathEscape},
		{"ErrInputTooLarge", domain.ErrInputTooLarge},
		{"ErrDuplicateAlias", domain.ErrDuplicateAlias},
		{"ErrDuplicatePath", domain.ErrDuplicatePath},
	}

	for _, tt := range sentinels {
		t.Run(tt.name+"/direct_match", func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("errors.Is(%s, %s) = false; want true", tt.name, tt.name)
			}
		})

		t.Run(tt.name+"/wrapped_match", func(t *testing.T) {
			t.Parallel()
			wrapped := fmt.Errorf("outer context: %w", tt.err)
			if !errors.Is(wrapped, tt.err) {
				t.Errorf("errors.Is(fmt.Errorf(\"%%w\", %s), %s) = false; want true", tt.name, tt.name)
			}
		})

		t.Run(tt.name+"/no_cross_match", func(t *testing.T) {
			t.Parallel()
			for _, other := range sentinels {
				if other.err == tt.err {
					continue
				}
				if errors.Is(tt.err, other.err) {
					t.Errorf("errors.Is(%s, %s) = true; want false — sentinel errors must be distinct",
						tt.name, other.name)
				}
			}
		})
	}
}
