package okf_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

func TestFileNodeRepository_ReadReserved_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	repo := okf.NewFileNodeRepository(map[string]string{"b": dir})
	_, err := repo.ReadReserved(context.Background(), "b", "../escape.md")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

func TestFileNodeRepository_WriteReserved_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	repo := okf.NewFileNodeRepository(map[string]string{"b": dir})
	err := repo.WriteReserved(context.Background(), "b", "../escape.md", "content")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

func TestFileNodeRepository_List_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	repo := okf.NewFileNodeRepository(map[string]string{"b": dir})
	_, err := repo.List(context.Background(), "b", "../")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

func TestFileNodeRepository_Get_RejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	// Write a 2 MB file (exceeds 1 MB cap).
	bigPath := filepath.Join(dir, "big.md")
	require.NoError(t, os.WriteFile(bigPath, make([]byte, 2<<20), 0o644)) //nolint:gosec // test helper

	repo := okf.NewFileNodeRepository(map[string]string{"b": dir})
	_, err := repo.Get(context.Background(), domain.ConceptRef{BundleAlias: "b", RelativePath: "big.md"})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInputTooLarge)
}
