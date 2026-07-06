//go:build integration

package okf_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/okf"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// TestFileNodeRepository_Integration_RoundTrip verifies Put→Get→List→ReadReserved
// on a real filesystem. Gated with //go:build integration.
func TestFileNodeRepository_Integration_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	repo := okf.NewFileNodeRepository(map[string]string{"b": dir})

	ref := domain.ConceptRef{BundleAlias: "b", RelativePath: "notes/idea.md"}
	concept := &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		Body:        "# Idea\n\nThis is a test concept.",
	}

	// Put
	err := repo.Put(context.Background(), ref, concept)
	require.NoError(t, err)

	// Get
	got, err := repo.Get(context.Background(), ref)
	require.NoError(t, err)
	assert.Equal(t, "note", got.Frontmatter.Type)
	assert.Contains(t, got.Body, "test concept")

	// WriteReserved + ReadReserved
	err = repo.WriteReserved(context.Background(), "b", "index.md", "# Index\n")
	require.NoError(t, err)
	content, err := repo.ReadReserved(context.Background(), "b", "index.md")
	require.NoError(t, err)
	assert.Equal(t, "# Index\n", content)

	// List
	refs, err := repo.List(context.Background(), "b", "")
	require.NoError(t, err)
	found := false
	for _, r := range refs {
		if r.RelativePath == filepath.Join("notes", "idea.md") {
			found = true
		}
	}
	assert.True(t, found, "List should include the created concept")

	// Containment guard: traversal is rejected
	_, err = repo.Get(context.Background(), domain.ConceptRef{
		BundleAlias:  "b",
		RelativePath: "../escape.md",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}
