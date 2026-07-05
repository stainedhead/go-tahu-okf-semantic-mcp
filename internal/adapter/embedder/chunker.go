package embedder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
)

const maxChunkRunes = 512

// ChunkConcept splits an OKFConcept into EmbeddingChunks ready for indexing.
//
// Chunk 0 is always a frontmatter summary text in the form:
//
//	"type: X | title: Y | description: Z | tags: [a,b,c]"
//
// Chunks 1+ are paragraph chunks from the concept body, split on blank lines
// ("\n\n"). Paragraphs longer than 512 runes are further split into
// 512-rune windows. Empty paragraphs are skipped.
//
// Each chunk ID has the format "bundleAlias:relPath:chunkIndex".
func ChunkConcept(concept *domain.OKFConcept, bundleAlias string) []domain.EmbeddingChunk {
	fm := concept.Frontmatter
	relPath := concept.Ref.RelativePath

	// "type:title" is the canonical display summary stored on every chunk.
	fmSummary := fm.Type + ":" + fm.Title

	makeChunk := func(idx int, text string) domain.EmbeddingChunk {
		return domain.EmbeddingChunk{
			ID:                 bundleAlias + ":" + relPath + ":" + strconv.Itoa(idx),
			BundleAlias:        bundleAlias,
			ConceptPath:        relPath,
			ChunkIndex:         idx,
			Text:               text,
			FrontmatterSummary: fmSummary,
		}
	}

	chunks := []domain.EmbeddingChunk{
		makeChunk(0, frontmatterText(fm)),
	}

	idx := 1
	for _, para := range strings.Split(concept.Body, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		runes := []rune(para)
		for start := 0; start < len(runes); start += maxChunkRunes {
			end := start + maxChunkRunes
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, makeChunk(idx, string(runes[start:end])))
			idx++
		}
	}

	return chunks
}

// frontmatterText builds the human-readable summary text for chunk 0.
func frontmatterText(fm domain.OKFFrontmatter) string {
	tags := "[" + strings.Join(fm.Tags, ",") + "]"
	return fmt.Sprintf("type: %s | title: %s | description: %s | tags: %s",
		fm.Type, fm.Title, fm.Description, tags)
}
