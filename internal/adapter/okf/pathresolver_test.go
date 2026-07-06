package okf_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

func TestBundlePathResolver_Resolve_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	r := okf.NewBundlePathResolver(map[string]string{"b": dir})

	_, err := r.Resolve("b", "../escape.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

func TestBundlePathResolver_Resolve_RejectsReservedNames(t *testing.T) {
	dir := t.TempDir()
	r := okf.NewBundlePathResolver(map[string]string{"b": dir})

	_, err := r.Resolve("b", "index.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrReservedPath)

	_, err = r.Resolve("b", "subdir/log.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrReservedPath)
}

func TestBundlePathResolver_Resolve_ValidPath(t *testing.T) {
	dir := t.TempDir()
	conceptPath := filepath.Join(dir, "notes", "concept.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(conceptPath), 0o755))      //nolint:gosec // test helper
	require.NoError(t, os.WriteFile(conceptPath, []byte("# test"), 0o644)) //nolint:gosec // test helper

	// On macOS /var → /private/var via symlink; resolve canonical path for comparison.
	canonicalPath, err := filepath.EvalSymlinks(conceptPath)
	require.NoError(t, err)

	r := okf.NewBundlePathResolver(map[string]string{"b": dir})
	got, err := r.Resolve("b", "notes/concept.md")
	require.NoError(t, err)
	assert.Equal(t, canonicalPath, got)
}

func TestBundlePathResolver_SymlinkEscape_Rejected(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.md")
	require.NoError(t, os.WriteFile(outsideFile, []byte("secret"), 0o644)) //nolint:gosec // test helper

	// Create a symlink inside the bundle pointing outside.
	symlinkPath := filepath.Join(dir, "escape.md")
	require.NoError(t, os.Symlink(outsideFile, symlinkPath))

	r := okf.NewBundlePathResolver(map[string]string{"b": dir})
	_, err := r.Resolve("b", "escape.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

func TestBundlePathResolver_ResolveReserved_ContainmentEnforced(t *testing.T) {
	dir := t.TempDir()
	r := okf.NewBundlePathResolver(map[string]string{"b": dir})

	_, err := r.ResolveReserved("b", "../escape.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

func TestBundlePathResolver_UnknownBundle_ReturnsNotFound(t *testing.T) {
	r := okf.NewBundlePathResolver(map[string]string{})
	_, err := r.Resolve("missing", "x.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
