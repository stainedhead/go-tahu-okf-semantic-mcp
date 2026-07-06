package domain

import (
	"fmt"
	"strings"
	"time"
)

// OKFFrontmatter holds the parsed YAML frontmatter for an OKF concept document.
// Type is required per OKF v0.1; all other fields are optional.
type OKFFrontmatter struct {
	Type        string // REQUIRED per OKF v0.1
	Title       string
	Description string
	Resource    string // URI of the described resource
	Tags        []string
	Timestamp   time.Time
	Extra       map[string]any // unknown keys preserved in insertion order
}

// Validate returns an error if the frontmatter violates OKF v0.1 invariants.
// Currently: Type must be non-empty.
func (f OKFFrontmatter) Validate() error {
	if f.Type == "" {
		return fmt.Errorf("domain: OKFFrontmatter.Type is required per OKF v0.1")
	}
	return nil
}

// OKFConcept is the in-memory representation of a parsed OKF document.
type OKFConcept struct {
	Ref           ConceptRef
	Frontmatter   OKFFrontmatter
	Body          string // raw markdown body (after frontmatter block)
	OutboundLinks []ConceptLink
}

// ConceptRef is a value object that uniquely identifies an OKF concept
// within a bundle.
type ConceptRef struct {
	BundleAlias  string // e.g. "my-kb"
	RelativePath string // e.g. "runbooks/deploy-pipeline.md"
}

// NewConceptRef constructs and validates a ConceptRef. Both alias and relPath
// must be non-empty; relPath must not contain ".." traversal segments.
func NewConceptRef(alias, relPath string) (ConceptRef, error) {
	if alias == "" {
		return ConceptRef{}, fmt.Errorf("domain: ConceptRef alias must not be empty")
	}
	if relPath == "" {
		return ConceptRef{}, fmt.Errorf("domain: ConceptRef relPath must not be empty")
	}
	for _, seg := range strings.Split(relPath, "/") {
		if seg == ".." {
			return ConceptRef{}, fmt.Errorf("domain: ConceptRef relPath contains traversal segment: %q", relPath)
		}
	}
	return ConceptRef{BundleAlias: alias, RelativePath: relPath}, nil
}

// String returns the canonical string representation "alias:relative/path.md".
func (r ConceptRef) String() string {
	return r.BundleAlias + ":" + r.RelativePath
}

// ConceptLink represents a single outbound hyperlink found in a concept body.
type ConceptLink struct {
	Target string // resolved relative path (e.g. "../apis/payments-api.md")
	Text   string // link display text
	Broken bool   // true if target does not exist on disk
}
