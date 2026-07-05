package embedder_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/adapter/embedder"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

// TestBM25Embedder_Embed_ReturnsSameDimForAllTexts verifies that every vector
// returned by Embed has the same length and that length equals Dims().
func TestBM25Embedder_Embed_ReturnsSameDimForAllTexts(t *testing.T) {
	e := embedder.New()
	e.Index("doc1", "the quick brown fox jumps over the lazy dog")
	e.Index("doc2", "a fast red fox leaps past a sleeping hound")
	e.Index("doc3", "semantic search with vector embeddings")

	ctx := context.Background()
	texts := []string{
		"fox and dog",
		"vector search",
		"completely unrelated words zephyr quasar",
		"",
	}

	vecs, err := e.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("expected %d vectors, got %d", len(texts), len(vecs))
	}

	dims := e.Dims()
	if dims == 0 {
		t.Fatal("Dims() returned 0 after indexing documents")
	}
	for i, v := range vecs {
		if len(v) != dims {
			t.Errorf("text[%d]: vector length %d, want %d (Dims)", i, len(v), dims)
		}
	}
}

// TestBM25Embedder_Embed_DifferentTextsProduceDifferentVectors verifies that
// semantically distinct texts yield distinct BM25 vectors.
func TestBM25Embedder_Embed_DifferentTextsProduceDifferentVectors(t *testing.T) {
	e := embedder.New()
	e.Index("doc1", "the quick brown fox jumps over the lazy dog")
	e.Index("doc2", "a fast red fox leaps past a sleeping hound")
	e.Index("doc3", "semantic search with vector embeddings and retrieval")

	ctx := context.Background()
	vecs, err := e.Embed(ctx, []string{
		"fox jumps dog",
		"semantic embeddings retrieval",
	})
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}

	same := true
	for i, v := range vecs[0] {
		if v != vecs[1][i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different vectors for different texts, got identical vectors")
	}
}

// TestChunkConcept_FrontmatterIsFirstChunk verifies that chunk 0 carries the
// frontmatter summary text and the canonical FrontmatterSummary display value.
func TestChunkConcept_FrontmatterIsFirstChunk(t *testing.T) {
	concept := &domain.OKFConcept{
		Ref: domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/idea.md"},
		Frontmatter: domain.OKFFrontmatter{
			Type:        "note",
			Title:       "My Idea",
			Description: "An interesting idea",
			Tags:        []string{"foo", "bar"},
		},
		Body: "This is the body.",
	}

	chunks := embedder.ChunkConcept(concept, "kb")
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk, got none")
	}

	first := chunks[0]

	if first.ChunkIndex != 0 {
		t.Errorf("first chunk index: want 0, got %d", first.ChunkIndex)
	}
	if first.ID != "kb:notes/idea.md:0" {
		t.Errorf("first chunk ID: want %q, got %q", "kb:notes/idea.md:0", first.ID)
	}
	if first.BundleAlias != "kb" {
		t.Errorf("BundleAlias: want %q, got %q", "kb", first.BundleAlias)
	}
	if first.ConceptPath != "notes/idea.md" {
		t.Errorf("ConceptPath: want %q, got %q", "notes/idea.md", first.ConceptPath)
	}

	for _, want := range []string{
		"type: note",
		"title: My Idea",
		"description: An interesting idea",
		"tags: [foo,bar]",
	} {
		if !strings.Contains(first.Text, want) {
			t.Errorf("frontmatter text missing %q; full text: %q", want, first.Text)
		}
	}

	if first.FrontmatterSummary != "note:My Idea" {
		t.Errorf("FrontmatterSummary: want %q, got %q", "note:My Idea", first.FrontmatterSummary)
	}
}

// TestChunkConcept_LongBodySplitsIntoParagraphs verifies that blank lines
// create separate chunks and that paragraphs exceeding 512 runes are further
// split into 512-rune windows.
func TestChunkConcept_LongBodySplitsIntoParagraphs(t *testing.T) {
	// 600-rune paragraph triggers a further split (512 + 88).
	longPara := strings.Repeat("a", 600)
	body := "Short paragraph.\n\n" + longPara

	concept := &domain.OKFConcept{
		Ref:         domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/big.md"},
		Frontmatter: domain.OKFFrontmatter{Type: "note", Title: "Big"},
		Body:        body,
	}

	chunks := embedder.ChunkConcept(concept, "kb")

	// Expected layout:
	//   chunks[0]: frontmatter
	//   chunks[1]: "Short paragraph."
	//   chunks[2]: first 512 runes of longPara
	//   chunks[3]: remaining 88 runes of longPara
	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 chunks, got %d", len(chunks))
	}

	// Chunk indices must be contiguous starting from 0.
	for i, c := range chunks {
		if c.ChunkIndex != i {
			t.Errorf("chunks[%d].ChunkIndex = %d, want %d", i, c.ChunkIndex, i)
		}
		wantID := "kb:notes/big.md:" + itoa(i)
		if c.ID != wantID {
			t.Errorf("chunks[%d].ID = %q, want %q", i, c.ID, wantID)
		}
	}

	if got := len([]rune(chunks[2].Text)); got != 512 {
		t.Errorf("chunks[2] rune length: want 512, got %d", got)
	}
	if got := len([]rune(chunks[3].Text)); got != 88 {
		t.Errorf("chunks[3] rune length: want 88, got %d", got)
	}
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 5)
	for n > 0 {
		buf = append([]byte{digits[n%10]}, buf...)
		n /= 10
	}
	return string(buf)
}
