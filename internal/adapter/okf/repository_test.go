package okf_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestFileNodeRepository_Put_RegeneratesIndex verifies that a successful Put
// regenerates index.md in the affected directory (spec FR-011).
func TestFileNodeRepository_Put_RegeneratesIndex(t *testing.T) {
	repo, dir := makeRepo(t)
	ctx := context.Background()

	concept := &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{
			Type:  "guide",
			Title: "My Guide",
		},
		Body: "# My Guide\n",
	}

	ref := domain.ConceptRef{BundleAlias: testAlias, RelativePath: "my-guide.md"}
	if err := repo.Put(ctx, ref, concept); err != nil {
		t.Fatalf("Put: %v", err)
	}

	indexPath := filepath.Join(dir, "index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "my-guide.md") {
		t.Errorf("index.md does not contain 'my-guide.md':\n%s", content)
	}
}

// TestFileNodeRepository_Put_AppendsLog verifies that a successful Put appends
// a timestamped entry to log.md, creating the file if absent (spec FR-011,
// edge case: log.md absent on first write).
func TestFileNodeRepository_Put_AppendsLog(t *testing.T) {
	repo, dir := makeRepo(t)
	ctx := context.Background()

	// Ensure log.md does not exist before the first write.
	logPath := filepath.Join(dir, "log.md")
	if _, err := os.Stat(logPath); err == nil {
		t.Fatal("log.md should not exist before first Put")
	}

	concept := &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{
			Type:  "note",
			Title: "Test Note",
		},
		Body: "# Test\n",
	}

	ref := domain.ConceptRef{BundleAlias: testAlias, RelativePath: "note.md"}
	if err := repo.Put(ctx, ref, concept); err != nil {
		t.Fatalf("Put: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log.md: %v", err)
	}

	content := string(data)
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(content, today) {
		t.Errorf("log.md does not contain today's date %s:\n%s", today, content)
	}
	if !strings.Contains(content, "note.md") {
		t.Errorf("log.md does not mention note.md:\n%s", content)
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
