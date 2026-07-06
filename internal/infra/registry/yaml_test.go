package registry_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/infra/registry"
)

func TestYAMLBundleRepository_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.yaml")
	repo := registry.NewYAMLBundleRepository(path)

	entry := domain.BundleEntry{
		Alias:     "test",
		RootPath:  "/tmp/test",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	require.NoError(t, repo.Put(context.Background(), entry))

	got, err := repo.Get(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, "test", got.Alias)
	assert.Equal(t, "/tmp/test", got.RootPath)

	bundles, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, bundles, 1)

	require.NoError(t, repo.Delete(context.Background(), "test"))
	_, err = repo.Get(context.Background(), "test")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestYAMLBundleRepository_GetNotFound(t *testing.T) {
	repo := registry.NewYAMLBundleRepository(filepath.Join(t.TempDir(), "reg.yaml"))
	_, err := repo.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestYAMLBundleRepository_ListEmpty(t *testing.T) {
	repo := registry.NewYAMLBundleRepository(filepath.Join(t.TempDir(), "reg.yaml"))
	bundles, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, bundles)
}
