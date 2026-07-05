package domain

import "time"

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
