package domain_test

import (
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// TestParseScope_FR_SpecSearch validates domain.ParseScope against all
// recognised scope formats and the error cases required by the spec.
// Spec: FR-012, FR-013, FR-014 — "scope" parameter grammar.
func TestParseScope_FR_SpecSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantKind domain.ScopeKind
		wantErr  bool
	}{
		// Happy paths
		{"global", domain.ScopeGlobal, false},
		{"bundle:kb", domain.ScopeBundle, false},
		{"bundle:my-knowledge-base", domain.ScopeBundle, false},
		{"path:kb:notes/deploy.md", domain.ScopePath, false},
		{"path:kb:deeply/nested/dir", domain.ScopePath, false},
		// Error paths
		{"invalid", domain.ScopeGlobal, true},
		{"", domain.ScopeGlobal, true},
		{"bundle:", domain.ScopeGlobal, true},            // empty alias
		{"path:kb", domain.ScopeGlobal, true},            // missing subpath separator
		{"path::notes/doc.md", domain.ScopeGlobal, true}, // empty alias
		{"path:kb:", domain.ScopeGlobal, true},           // empty subpath
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := domain.ParseScope(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseScope(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseScope(%q): unexpected error: %v", tt.input, err)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("ParseScope(%q): Kind = %v, want %v", tt.input, got.Kind, tt.wantKind)
			}
		})
	}
}

// TestParseScope_BundleScope_Fields verifies that the BundleAlias field is
// correctly populated for a bundle-scoped parse result.
func TestParseScope_BundleScope_Fields(t *testing.T) {
	t.Parallel()
	got, err := domain.ParseScope("bundle:my-kb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != domain.ScopeBundle {
		t.Errorf("Kind = %v, want ScopeBundle", got.Kind)
	}
	if got.BundleAlias != "my-kb" {
		t.Errorf("BundleAlias = %q, want %q", got.BundleAlias, "my-kb")
	}
}

// TestParseScope_PathScope_Fields verifies that both BundleAlias and SubPath
// are correctly populated for a path-scoped parse result.
func TestParseScope_PathScope_Fields(t *testing.T) {
	t.Parallel()
	got, err := domain.ParseScope("path:kb:notes/deploy.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != domain.ScopePath {
		t.Errorf("Kind = %v, want ScopePath", got.Kind)
	}
	if got.BundleAlias != "kb" {
		t.Errorf("BundleAlias = %q, want %q", got.BundleAlias, "kb")
	}
	if got.SubPath != "notes/deploy.md" {
		t.Errorf("SubPath = %q, want %q", got.SubPath, "notes/deploy.md")
	}
}

// TestConceptRef_String verifies the canonical "alias:relative/path.md"
// string representation of a ConceptRef.
func TestConceptRef_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		alias, relPath, want string
	}{
		{"kb", "notes/doc.md", "kb:notes/doc.md"},
		{"my-kb", "index.md", "my-kb:index.md"},
		{"x", "a/b/c.md", "x:a/b/c.md"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			ref := domain.ConceptRef{BundleAlias: tt.alias, RelativePath: tt.relPath}
			if got := ref.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
