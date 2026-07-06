package registry_test

import (
	"context"
	"os"
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

func TestYAMLBundleRepository_PutMultiple(t *testing.T) {
	repo := registry.NewYAMLBundleRepository(filepath.Join(t.TempDir(), "reg.yaml"))
	ctx := context.Background()

	for _, alias := range []string{"a", "b", "c"} {
		require.NoError(t, repo.Put(ctx, domain.BundleEntry{Alias: alias, RootPath: "/tmp/" + alias}))
	}

	bundles, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, bundles, 3)
}

func TestYAMLBundleRepository_PutUpdate(t *testing.T) {
	repo := registry.NewYAMLBundleRepository(filepath.Join(t.TempDir(), "reg.yaml"))
	ctx := context.Background()

	require.NoError(t, repo.Put(ctx, domain.BundleEntry{Alias: "kb", RootPath: "/tmp/v1"}))
	require.NoError(t, repo.Put(ctx, domain.BundleEntry{Alias: "kb", RootPath: "/tmp/v2"}))

	got, err := repo.Get(ctx, "kb")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/v2", got.RootPath)
}

func TestYAMLBundleRepository_DeleteMissing(t *testing.T) {
	repo := registry.NewYAMLBundleRepository(filepath.Join(t.TempDir(), "reg.yaml"))
	// Deleting a non-existent alias is a no-op (not an error).
	require.NoError(t, repo.Delete(context.Background(), "missing"))
}

func TestYAMLBundleRepository_CorruptYAML_ReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: valid: yaml: [\n"), 0o600))

	repo := registry.NewYAMLBundleRepository(path)
	_, err := repo.Get(context.Background(), "any")
	require.Error(t, err)
}
