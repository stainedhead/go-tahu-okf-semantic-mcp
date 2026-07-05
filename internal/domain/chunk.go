package domain

import (
	"fmt"
	"strings"
)

// EmbeddingChunk is a text chunk with its vector embedding, stored in the
// vector index. ID format: "alias:path:chunk_index".
type EmbeddingChunk struct {
	ID                 string // "alias:path:chunk_index"
	BundleAlias        string
	ConceptPath        string
	ChunkIndex         int
	Text               string
	Embedding          []float32
	FrontmatterSummary string // "type:title" for display
}

// ScoredChunk is a retrieved chunk with its similarity score.
type ScoredChunk struct {
	Source             string // "alias:path"
	ChunkIndex         int
	ChunkText          string
	Score              float32
	FrontmatterSummary string
}

// ScopeKind enumerates the supported search scope levels.
type ScopeKind int

// ScopeKind constant values for the three supported search scopes.
const (
	ScopeGlobal ScopeKind = iota // search across all bundles
	ScopeBundle                  // restrict to one bundle
	ScopePath                    // restrict to a sub-path within a bundle
)

// Scope describes the search boundary for vector and keyword queries.
type Scope struct {
	Kind        ScopeKind
	BundleAlias string // set for ScopeBundle and ScopePath
	SubPath     string // set for ScopePath only
}

// ParseScope parses a scope string in one of three formats:
//   - "global"
//   - "bundle:<alias>"
//   - "path:<alias>:<subpath>"
func ParseScope(s string) (Scope, error) {
	switch {
	case s == "global":
		return Scope{Kind: ScopeGlobal}, nil

	case strings.HasPrefix(s, "bundle:"):
		alias := strings.TrimPrefix(s, "bundle:")
		if alias == "" {
			return Scope{}, fmt.Errorf("bundle scope requires a non-empty alias")
		}
		return Scope{Kind: ScopeBundle, BundleAlias: alias}, nil

	case strings.HasPrefix(s, "path:"):
		rest := strings.TrimPrefix(s, "path:")
		idx := strings.Index(rest, ":")
		if idx < 0 {
			return Scope{}, fmt.Errorf("path scope requires format path:<alias>:<subpath>")
		}
		alias := rest[:idx]
		subPath := rest[idx+1:]
		if alias == "" {
			return Scope{}, fmt.Errorf("path scope requires a non-empty alias")
		}
		return Scope{Kind: ScopePath, BundleAlias: alias, SubPath: subPath}, nil

	default:
		return Scope{}, fmt.Errorf("unrecognised scope %q: must be \"global\", \"bundle:<alias>\", or \"path:<alias>:<subpath>\"", s)
	}
}
