package okf_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// TestFileNodeRepository_List_BasicListing verifies that List returns .md files
// below a subpath, excluding reserved files.
func TestFileNodeRepository_List_BasicListing(t *testing.T) {
	t.Parallel()
	repo, dir := makeRepo(t)

	writeFile(t, dir, "a.md", "---\ntype: note\n---\nbody")
	writeFile(t, dir, "sub/b.md", "---\ntype: note\n---\nbody")
	writeFile(t, dir, "index.md", "# reserved")
	writeFile(t, dir, "log.md", "# reserved")

	refs, err := repo.List(context.Background(), testAlias, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	paths := make(map[string]bool)
	for _, r := range refs {
		paths[r.RelativePath] = true
	}
	if !paths["a.md"] {
		t.Error("expected a.md in List result")
	}
	if !paths["sub/b.md"] {
		t.Error("expected sub/b.md in List result")
	}
	if paths["index.md"] {
		t.Error("index.md must be excluded from List")
	}
	if paths["log.md"] {
		t.Error("log.md must be excluded from List")
	}
}

// TestFileNodeRepository_List_SubPath verifies filtering by subPath.
func TestFileNodeRepository_List_SubPath(t *testing.T) {
	t.Parallel()
	repo, dir := makeRepo(t)

	writeFile(t, dir, "root.md", "---\ntype: note\n---\nbody")
	writeFile(t, dir, "sub/nested.md", "---\ntype: note\n---\nbody")

	refs, err := repo.List(context.Background(), testAlias, "sub")
	if err != nil {
		t.Fatalf("List(sub): %v", err)
	}
	for _, r := range refs {
		if r.RelativePath == "root.md" {
			t.Error("root.md must not appear when listing under 'sub'")
		}
	}
}

// TestFileNodeRepository_List_UnknownBundle returns ErrNotFound.
func TestFileNodeRepository_List_UnknownBundle(t *testing.T) {
	t.Parallel()
	repo := okf.NewFileNodeRepository(map[string]string{})
	_, err := repo.List(context.Background(), "missing", "")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("List(missing alias): got %v, want ErrNotFound", err)
	}
}

// TestFileNodeRepository_ListTypes_ReturnsTypes verifies that ListTypes returns
// the unique set of concept types in the bundle.
func TestFileNodeRepository_ListTypes_ReturnsTypes(t *testing.T) {
	t.Parallel()
	repo, dir := makeRepo(t)

	writeFile(t, dir, "a.md", "---\ntype: note\n---\nbody")
	writeFile(t, dir, "b.md", "---\ntype: note\n---\nbody")
	writeFile(t, dir, "c.md", "---\ntype: runbook\n---\nbody")

	types, err := repo.ListTypes(context.Background(), testAlias)
	if err != nil {
		t.Fatalf("ListTypes: %v", err)
	}

	typesSet := make(map[string]bool)
	for _, typ := range types {
		typesSet[typ] = true
	}
	if !typesSet["note"] {
		t.Error("expected 'note' type in ListTypes result")
	}
	if !typesSet["runbook"] {
		t.Error("expected 'runbook' type in ListTypes result")
	}
}

// TestFileNodeRepository_ListTypes_UnknownBundle returns ErrNotFound.
func TestFileNodeRepository_ListTypes_UnknownBundle(t *testing.T) {
	t.Parallel()
	repo := okf.NewFileNodeRepository(map[string]string{})
	_, err := repo.ListTypes(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ListTypes(missing): got %v, want ErrNotFound", err)
	}
}

// TestFileNodeRepository_ReadReserved_Basic verifies round-trip via WriteReserved+ReadReserved.
func TestFileNodeRepository_ReadReserved_Basic(t *testing.T) {
	t.Parallel()
	repo, _ := makeRepo(t)

	const content = "# index\n\nsome content\n"
	err := repo.WriteReserved(context.Background(), testAlias, "index.md", content)
	if err != nil {
		t.Fatalf("WriteReserved: %v", err)
	}

	got, err := repo.ReadReserved(context.Background(), testAlias, "index.md")
	if err != nil {
		t.Fatalf("ReadReserved: %v", err)
	}
	if got != content {
		t.Errorf("ReadReserved = %q, want %q", got, content)
	}
}

// TestFileNodeRepository_ReadReserved_MissingFile returns ErrNotFound.
func TestFileNodeRepository_ReadReserved_MissingFile(t *testing.T) {
	t.Parallel()
	repo, _ := makeRepo(t)
	_, err := repo.ReadReserved(context.Background(), testAlias, "log.md")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("ReadReserved(missing): got %v, want ErrNotFound", err)
	}
}

// TestBundlePathResolver_RejectsTraversal exercises traversal rejection via BundlePathResolver.
func TestBundlePathResolver_RejectsTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := okf.NewBundlePathResolver(map[string]string{"b": dir})

	_, err := r.Resolve("b", "../escape.md")
	if !errors.Is(err, domain.ErrPathEscape) {
		t.Errorf("Resolve traversal: got %v, want ErrPathEscape", err)
	}
}

// TestBundlePathResolver_RejectsReserved verifies index.md and log.md are blocked.
func TestBundlePathResolver_RejectsReservedViaRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := okf.NewBundlePathResolver(map[string]string{"b": dir})

	for _, reserved := range []string{"index.md", "log.md", "sub/index.md"} {
		_, err := r.Resolve("b", reserved)
		if !errors.Is(err, domain.ErrReservedPath) {
			t.Errorf("Resolve(%q): got %v, want ErrReservedPath", reserved, err)
		}
	}
}

// TestBundlePathResolver_ValidPath verifies normal paths are accepted.
func TestBundlePathResolver_ValidPathViaRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := okf.NewBundlePathResolver(map[string]string{"b": dir})
	_, err := r.Resolve("b", "sub/note.md")
	// ErrNotExist is expected since the file doesn't actually exist on disk;
	// ErrPathEscape/ErrReservedPath would indicate a validation failure.
	if errors.Is(err, domain.ErrPathEscape) || errors.Is(err, domain.ErrReservedPath) {
		t.Errorf("Resolve(valid path): got unexpected validation error: %v", err)
	}
}
