package domain_test

import (
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// TestNewConceptRef_Validates checks the validating constructor (P5.1/FR-029).
func TestNewConceptRef_Validates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		alias   string
		relPath string
		wantErr bool
		name    string
	}{
		{"kb", "notes/deploy.md", false, "valid"},
		{"", "notes/doc.md", true, "empty alias"},
		{"kb", "", true, "empty relPath"},
		{"kb", "../etc/passwd", true, "traversal in relPath"},
		{"kb", "a/../b/file.md", true, "mid-path traversal"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ref, err := domain.NewConceptRef(tt.alias, tt.relPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewConceptRef(%q, %q): expected error, got nil (ref=%v)", tt.alias, tt.relPath, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewConceptRef(%q, %q): unexpected error: %v", tt.alias, tt.relPath, err)
			}
			if ref.BundleAlias != tt.alias {
				t.Errorf("BundleAlias = %q, want %q", ref.BundleAlias, tt.alias)
			}
			if ref.RelativePath != tt.relPath {
				t.Errorf("RelativePath = %q, want %q", ref.RelativePath, tt.relPath)
			}
		})
	}
}

// TestOKFFrontmatter_Validate_RequiresType checks Validate returns an error
// when Type is empty (P5.2/FR-030).
func TestOKFFrontmatter_Validate_RequiresType(t *testing.T) {
	t.Parallel()

	t.Run("empty type returns error", func(t *testing.T) {
		t.Parallel()
		fm := domain.OKFFrontmatter{}
		if err := fm.Validate(); err == nil {
			t.Fatal("Validate(): expected error for empty Type, got nil")
		}
	})

	t.Run("non-empty type passes", func(t *testing.T) {
		t.Parallel()
		fm := domain.OKFFrontmatter{Type: "concept"}
		if err := fm.Validate(); err != nil {
			t.Fatalf("Validate(): unexpected error: %v", err)
		}
	})
}
