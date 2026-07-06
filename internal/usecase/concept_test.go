package usecase_test

// Fakes for domain.NodeRepository and domain.BundleRepository are declared in
// bundle_test.go (same package), so this file only contains helpers and tests.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newConceptService constructs a ConceptService wired with in-memory fakes.
func newConceptService(nr *fakeNodeRepo, br *fakeBundleRepo) *usecase.ConceptService {
	return &usecase.ConceptService{
		NodeRepository:   nr,
		BundleRepository: br,
	}
}

// makeConcept constructs a minimal OKFConcept for use in tests.
// Timestamp is left as zero — tests that care about it set it explicitly.
func makeConcept(kind, title, body string, links []domain.ConceptLink) *domain.OKFConcept {
	return &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{
			Type:  kind,
			Title: title,
		},
		Body:          body,
		OutboundLinks: links,
	}
}

// ---------------------------------------------------------------------------
// FR-005: concept_read
// ---------------------------------------------------------------------------

func TestConceptService_ReadConcept_FR005(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/hello.md"}
	want := makeConcept("note", "Hello", "body text", nil)
	_ = nr.Put(context.Background(), ref, want)

	got, err := svc.ReadConcept(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Frontmatter.Type != want.Frontmatter.Type {
		t.Errorf("type: got %q, want %q", got.Frontmatter.Type, want.Frontmatter.Type)
	}
	if got.Body != want.Body {
		t.Errorf("body: got %q, want %q", got.Body, want.Body)
	}
}

func TestConceptService_ReadConcept_NotFound_FR005(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "missing.md"}

	_, err := svc.ReadConcept(context.Background(), ref)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FR-006: concept_list
// ---------------------------------------------------------------------------

func TestConceptService_ListConcepts_WithEntries_FR006(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	_ = nr.Put(context.Background(),
		domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/a.md"},
		makeConcept("note", "A", "", nil))
	_ = nr.Put(context.Background(),
		domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/b.md"},
		makeConcept("note", "B", "", nil))

	refs, err := svc.ListConcepts(context.Background(), "kb", "notes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("expected 2 refs; got %d", len(refs))
	}
}

// TestConceptService_ListConcepts_EmptyPath_FR006 verifies that an empty
// subPath on an empty bundle returns a non-nil empty slice, not an error.
func TestConceptService_ListConcepts_EmptyPath_FR006(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())

	refs, err := svc.ListConcepts(context.Background(), "kb", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refs == nil {
		t.Error("expected non-nil empty slice; got nil")
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs; got %d", len(refs))
	}
}

// ---------------------------------------------------------------------------
// FR-007: concept_links
// ---------------------------------------------------------------------------

func TestConceptService_GetLinks_WithBrokenLinks_FR007(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	links := []domain.ConceptLink{
		{Target: "existing.md", Text: "Exists", Broken: false},
		{Target: "missing.md", Text: "Gone", Broken: true},
	}
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "doc.md"}
	_ = nr.Put(context.Background(), ref, makeConcept("note", "Doc", "", links))

	got, err := svc.GetLinks(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 links; got %d", len(got))
	}
	var brokenCount int
	for _, l := range got {
		if l.Broken {
			brokenCount++
		}
	}
	if brokenCount != 1 {
		t.Errorf("expected 1 broken link; got %d", brokenCount)
	}
}

// ---------------------------------------------------------------------------
// FR-008: index_read
// ---------------------------------------------------------------------------

func TestConceptService_ReadIndex_Found_FR008(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	_ = nr.WriteReserved(context.Background(), "kb", "notes/index.md", "# Index\n")

	content, err := svc.ReadIndex(context.Background(), "kb", "notes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "# Index") {
		t.Errorf("expected index content; got %q", content)
	}
}

func TestConceptService_ReadIndex_NotFound_FR008(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())

	_, err := svc.ReadIndex(context.Background(), "kb", "nonexistent")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FR-009: log_read
// ---------------------------------------------------------------------------

func TestConceptService_ReadLog_Found_FR009(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	_ = nr.WriteReserved(context.Background(), "kb", "log.md", "- entry\n")

	content, err := svc.ReadLog(context.Background(), "kb", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "- entry") {
		t.Errorf("expected log entry in content; got %q", content)
	}
}

func TestConceptService_ReadLog_NotFound_FR009(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())

	_, err := svc.ReadLog(context.Background(), "kb", "nonexistent")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FR-010: concept_type_list
// ---------------------------------------------------------------------------

func TestConceptService_ListTypes_FR010(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	_ = nr.Put(context.Background(),
		domain.ConceptRef{BundleAlias: "kb", RelativePath: "a.md"},
		makeConcept("note", "A", "", nil))
	_ = nr.Put(context.Background(),
		domain.ConceptRef{BundleAlias: "kb", RelativePath: "b.md"},
		makeConcept("runbook", "B", "", nil))
	// "note" appears twice; must deduplicate to 2 distinct types.
	_ = nr.Put(context.Background(),
		domain.ConceptRef{BundleAlias: "kb", RelativePath: "c.md"},
		makeConcept("note", "C", "", nil))

	types, err := svc.ListTypes(context.Background(), "kb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 2 {
		t.Errorf("expected 2 distinct types; got %d: %v", len(types), types)
	}
}

// ---------------------------------------------------------------------------
// FR-011: concept_write
// ---------------------------------------------------------------------------

func TestConceptService_WriteConcept_Happy_FR011(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/deploy.md"}
	concept := makeConcept("runbook", "Deploy Pipeline", "## Steps\n1. build\n", nil)

	if err := svc.WriteConcept(context.Background(), ref, concept); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Concept must be persisted and readable.
	got, err := svc.ReadConcept(context.Background(), ref)
	if err != nil {
		t.Fatalf("ReadConcept after write: %v", err)
	}
	if got.Frontmatter.Type != "runbook" {
		t.Errorf("type: got %q, want %q", got.Frontmatter.Type, "runbook")
	}

	// index.md must have been regenerated for the containing directory.
	idx, err := svc.ReadIndex(context.Background(), "kb", "notes")
	if err != nil {
		t.Fatalf("ReadIndex after write: %v", err)
	}
	if idx == "" {
		t.Error("expected non-empty index.md after write")
	}

	// log.md must contain a timestamped write entry.
	logContent, err := svc.ReadLog(context.Background(), "kb", "notes")
	if err != nil {
		t.Fatalf("ReadLog after write: %v", err)
	}
	if !strings.Contains(logContent, "concept_write") {
		t.Errorf("log.md does not contain expected entry; got: %q", logContent)
	}
}

func TestConceptService_WriteConcept_RejectsMissingType_FR011(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())
	ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/no-type.md"}

	err := svc.WriteConcept(context.Background(), ref, makeConcept("", "No Type", "body", nil))
	if !errors.Is(err, domain.ErrMissingType) {
		t.Fatalf("expected ErrMissingType; got %v", err)
	}
}

func TestConceptService_WriteConcept_RejectsReservedPath_FR011(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())

	cases := []struct {
		name string
		path string
	}{
		{"root index", "index.md"},
		{"root log", "log.md"},
		{"nested index", "notes/index.md"},
		{"nested log", "notes/log.md"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: tc.path}
			err := svc.WriteConcept(context.Background(), ref, makeConcept("note", "Reserved", "body", nil))
			if !errors.Is(err, domain.ErrReservedPath) {
				t.Errorf("expected ErrReservedPath for %q; got %v", tc.path, err)
			}
		})
	}
}

// TestConceptService_WriteConcept_IndexContainsTypeAndTitle_FIX003 asserts
// that regenerateIndex populates the Type and Title columns from frontmatter.
func TestConceptService_WriteConcept_IndexContainsTypeAndTitle_FIX003(t *testing.T) {
	t.Parallel()
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	ref1 := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/alpha.md"}
	ref2 := domain.ConceptRef{BundleAlias: "kb", RelativePath: "notes/beta.md"}

	if err := svc.WriteConcept(context.Background(), ref1, makeConcept("runbook", "Alpha Title", "body1", nil)); err != nil {
		t.Fatalf("WriteConcept ref1: %v", err)
	}
	if err := svc.WriteConcept(context.Background(), ref2, makeConcept("reference", "Beta Title", "body2", nil)); err != nil {
		t.Fatalf("WriteConcept ref2: %v", err)
	}

	idx, err := svc.ReadIndex(context.Background(), "kb", "notes")
	if err != nil {
		t.Fatalf("ReadIndex: %v", err)
	}
	for _, want := range []string{"runbook", "reference", "Alpha Title", "Beta Title"} {
		if !strings.Contains(idx, want) {
			t.Errorf("index.md missing %q; got:\n%s", want, idx)
		}
	}
}

// TestConceptService_ConcurrentWrite_LogPreservesAllEntries_FIX004 asserts
// that concurrent WriteConcept calls in the same directory produce exactly N
// log entries with no race (spec: concurrent writes must not cause data loss).
func TestConceptService_ConcurrentWrite_LogPreservesAllEntries_FIX004(t *testing.T) {
	nr := newFakeNodeRepo()
	br := newFakeBundleRepo()
	svc := newConceptService(nr, br)

	const N = 20
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			ref := domain.ConceptRef{
				BundleAlias:  "kb",
				RelativePath: fmt.Sprintf("notes/concept-%02d.md", i),
			}
			if err := svc.WriteConcept(context.Background(), ref,
				makeConcept("note", fmt.Sprintf("Title %d", i), "body", nil)); err != nil {
				t.Errorf("WriteConcept %d: %v", i, err)
			}
		}()
	}
	wg.Wait()

	logContent, err := svc.ReadLog(context.Background(), "kb", "notes")
	if err != nil {
		t.Fatalf("ReadLog: %v", err)
	}
	count := strings.Count(logContent, "concept_write")
	if count != N {
		t.Errorf("expected %d log entries, got %d\nlog:\n%s", N, count, logContent)
	}
}

// TestConceptService_WriteConcept_RejectsTraversal validates that WriteConcept
// rejects a RelativePath containing ".." components as defense-in-depth at the
// use-case boundary.
func TestConceptService_WriteConcept_RejectsTraversal(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())
	ref := domain.ConceptRef{BundleAlias: "b", RelativePath: "../escape.md"}
	err := svc.WriteConcept(context.Background(), ref, &domain.OKFConcept{
		Frontmatter: domain.OKFFrontmatter{Type: "note"},
		Body:        "content",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrPathEscape)
}

// TestConceptService_WriteConcept_AllowsNonReservedBasename_FR011 guards
// against a strings.HasSuffix implementation that would wrongly reject
// "changelog.md" (ends in "log.md") or "deploy-index.md" (ends in "index.md").
// Only the exact base name must be rejected.
func TestConceptService_WriteConcept_AllowsNonReservedBasename_FR011(t *testing.T) {
	t.Parallel()
	svc := newConceptService(newFakeNodeRepo(), newFakeBundleRepo())

	cases := []struct {
		name string
		path string
	}{
		{"changelog suffix", "notes/changelog.md"},
		{"deploy-index suffix", "notes/deploy-index.md"},
		{"root level concept", "root-concept.md"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref := domain.ConceptRef{BundleAlias: "kb", RelativePath: tc.path}
			err := svc.WriteConcept(context.Background(), ref, makeConcept("note", "OK", "body", nil))
			if err != nil {
				t.Errorf("expected no error for %q; got %v", tc.path, err)
			}
		})
	}
}
