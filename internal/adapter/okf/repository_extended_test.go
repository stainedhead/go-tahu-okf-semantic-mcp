package okf_test

import (
	"context"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
