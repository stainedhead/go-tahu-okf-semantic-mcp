package okf_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

const testAlias = "kb"

// makeRepo creates a temporary directory and a FileNodeRepository rooted there.
func makeRepo(t *testing.T) (*okf.FileNodeRepository, string) {
	t.Helper()
	dir := t.TempDir()
	repo := okf.NewFileNodeRepository(map[string]string{testAlias: dir})
	return repo, dir
}

// writeFile creates parent directories and writes content to a path within dir.
func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	absPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(absPath), err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", absPath, err)
	}
}

// TestFileNodeRepository_Get_ReturnsConceptWithLinks verifies that Get parses
// frontmatter fields and extracts outbound links from the markdown body,
// flagging broken links correctly (spec FR-005, FR-007).
func TestFileNodeRepository_Get_ReturnsConceptWithLinks(t *testing.T) {
	repo, dir := makeRepo(t)

	// Write the link target so it exists on disk.
	writeFile(t, dir, "other.md", "---\ntype: note\ntitle: Other\n---\n# Other\n")

	conceptContent := "---\ntype: guide\ntitle: Hello\n---\n# Hello\n\n[Other](other.md)\n[Missing](missing.md)\n"
	writeFile(t, dir, "hello.md", conceptContent)

	ref := domain.ConceptRef{BundleAlias: testAlias, RelativePath: "hello.md"}
	ctx := context.Background()

	concept, err := repo.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if concept.Frontmatter.Type != "guide" {
		t.Errorf("Type = %q, want %q", concept.Frontmatter.Type, "guide")
	}
	if concept.Frontmatter.Title != "Hello" {
		t.Errorf("Title = %q, want %q", concept.Frontmatter.Title, "Hello")
	}

	if len(concept.OutboundLinks) != 2 {
		t.Fatalf("OutboundLinks count = %d, want 2", len(concept.OutboundLinks))
	}

	// Find links by target name.
	linkMap := make(map[string]domain.ConceptLink, 2)
	for _, l := range concept.OutboundLinks {
		linkMap[l.Target] = l
	}

	if l, ok := linkMap["other.md"]; !ok {
		t.Error("link to other.md not found")
	} else {
		if l.Broken {
			t.Error("link to other.md should not be broken")
		}
		if l.Text != "Other" {
			t.Errorf("link text = %q, want %q", l.Text, "Other")
		}
	}

	if l, ok := linkMap["missing.md"]; !ok {
		t.Error("link to missing.md not found")
	} else if !l.Broken {
		t.Error("link to missing.md should be broken")
	}
}

// TestFileNodeRepository_Put_RejectsReservedPath_FR011 verifies that writing to
// index.md or log.md (at any directory level) returns domain.ErrReservedPath
// (spec FR-011).
func TestFileNodeRepository_Put_RejectsReservedPath_FR011(t *testing.T) {
	repo, _ := makeRepo(t)
	ctx := context.Background()

	concept := &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		Body:        "# Reserved path test\n",
	}

	reservedPaths := []string{
		"index.md",
		"log.md",
		"subdir/index.md",
		"subdir/log.md",
	}

	for _, p := range reservedPaths {
		ref := domain.ConceptRef{BundleAlias: testAlias, RelativePath: p}
		err := repo.Put(ctx, ref, concept)
		if !errors.Is(err, domain.ErrReservedPath) {
			t.Errorf("Put(%q) error = %v, want ErrReservedPath", p, err)
		}
	}
}

// TestFileNodeRepository_Put_RejectsMissingType_FR011 verifies that Put rejects
// a concept whose frontmatter has an empty type field (spec FR-011).
func TestFileNodeRepository_Put_RejectsMissingType_FR011(t *testing.T) {
	repo, _ := makeRepo(t)
	ctx := context.Background()

	concept := &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{Type: ""},
		Body:        "# Missing type test\n",
	}

	ref := domain.ConceptRef{BundleAlias: testAlias, RelativePath: "concept.md"}
	err := repo.Put(ctx, ref, concept)
	if !errors.Is(err, domain.ErrMissingType) {
		t.Errorf("Put with empty type: error = %v, want ErrMissingType", err)
	}
}

// TestFileNodeRepository_Put_WritesOnlyConceptFile_FIX002 verifies that Put
// writes only the concept file and does NOT create index.md or log.md.
// Index/log regeneration is the use case layer's responsibility (FIX-002).
func TestFileNodeRepository_Put_WritesOnlyConceptFile_FIX002(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := okf.NewFileNodeRepository(map[string]string{"kb": dir})
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "note.md"}
	concept := &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{Type: "note", Title: "Test Note"},
		Body:        "test body",
	}

	if err := repo.Put(context.Background(), ref, concept); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// The concept file must exist.
	if _, err := os.Stat(filepath.Join(dir, "note.md")); err != nil {
		t.Errorf("concept file not created: %v", err)
	}
	// index.md must NOT exist — Put should not create it; that's the use case's job.
	if _, err := os.Stat(filepath.Join(dir, "index.md")); err == nil {
		t.Error("FIX-002: Put must not create index.md; regenerateIndex in use case is responsible")
	}
	// log.md must NOT exist — Put should not create it; that's the use case's job.
	if _, err := os.Stat(filepath.Join(dir, "log.md")); err == nil {
		t.Error("FIX-002: Put must not create log.md; appendLog in use case is responsible")
	}
}

// TestFileNodeRepository_Get_PathEscape_FR019 verifies that a ConceptRef whose
// relative path escapes the bundle root returns domain.ErrPathEscape (spec FR-019).
func TestFileNodeRepository_Get_PathEscape_FR019(t *testing.T) {
	repo, _ := makeRepo(t)
	ctx := context.Background()

	escapingPaths := []string{
		"../../etc/passwd",
		"../outside.md",
		"subdir/../../outside.md",
	}

	for _, p := range escapingPaths {
		ref := domain.ConceptRef{BundleAlias: testAlias, RelativePath: p}
		_, err := repo.Get(ctx, ref)
		if !errors.Is(err, domain.ErrPathEscape) {
			t.Errorf("Get(%q) error = %v, want ErrPathEscape", p, err)
		}
	}
}
