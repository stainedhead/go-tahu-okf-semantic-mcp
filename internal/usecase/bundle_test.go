package usecase_test

// fakeEmbedder and fakeVectorStore are declared in search_test.go (same package).
// This file owns fakeBundleRepo and fakeNodeRepo, which are also used by concept_test.go.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/domain"
	"github.com/stainedhead/go-tahu-okf-semantic-mcp/internal/usecase"
)

// ---------------------------------------------------------------------------
// Compile-time interface conformance checks.
// ---------------------------------------------------------------------------

var (
	_ domain.BundleRepository = (*fakeBundleRepo)(nil)
	_ domain.NodeRepository   = (*fakeNodeRepo)(nil)
)

// ---------------------------------------------------------------------------
// fakeBundleRepo — in-memory BundleRepository
// ---------------------------------------------------------------------------

type fakeBundleRepo struct {
	mu      sync.RWMutex
	bundles map[string]domain.BundleEntry
}

func newFakeBundleRepo() *fakeBundleRepo {
	return &fakeBundleRepo{bundles: make(map[string]domain.BundleEntry)}
}

func (f *fakeBundleRepo) Get(_ context.Context, alias string) (*domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.bundles[alias]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &e, nil
}

func (f *fakeBundleRepo) Put(_ context.Context, entry domain.BundleEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[entry.Alias] = entry
	return nil
}

func (f *fakeBundleRepo) Delete(_ context.Context, alias string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bundles, alias)
	return nil
}

func (f *fakeBundleRepo) List(_ context.Context) ([]domain.BundleEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]domain.BundleEntry, 0, len(f.bundles))
	for _, e := range f.bundles {
		out = append(out, e)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// fakeNodeRepo — in-memory NodeRepository
// ---------------------------------------------------------------------------

type fakeNodeRepo struct {
	mu       sync.RWMutex
	concepts map[string]*domain.OKFConcept
	reserved map[string]string
}

func newFakeNodeRepo() *fakeNodeRepo {
	return &fakeNodeRepo{
		concepts: make(map[string]*domain.OKFConcept),
		reserved: make(map[string]string),
	}
}

func (f *fakeNodeRepo) Get(_ context.Context, ref domain.ConceptRef) (*domain.OKFConcept, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.concepts[ref.String()]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeNodeRepo) Put(_ context.Context, ref domain.ConceptRef, concept *domain.OKFConcept) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.concepts[ref.String()] = concept
	return nil
}

func (f *fakeNodeRepo) List(_ context.Context, bundleAlias, subPath string) ([]domain.ConceptRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	bundlePrefix := bundleAlias + ":"
	pathPrefix := bundlePrefix
	if subPath != "" {
		pathPrefix = bundlePrefix + subPath
	}
	var refs []domain.ConceptRef
	for key := range f.concepts {
		if strings.HasPrefix(key, pathPrefix) {
			refs = append(refs, domain.ConceptRef{
				BundleAlias:  bundleAlias,
				RelativePath: strings.TrimPrefix(key, bundlePrefix),
			})
		}
	}
	return refs, nil
}

func (f *fakeNodeRepo) ListTypes(_ context.Context, bundleAlias string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	prefix := bundleAlias + ":"
	seen := make(map[string]struct{})
	for key, c := range f.concepts {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if t := c.Frontmatter.Type; t != "" {
			seen[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeNodeRepo) ReadReserved(_ context.Context, bundleAlias, relPath string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	content, ok := f.reserved[bundleAlias+":"+relPath]
	if !ok {
		return "", domain.ErrNotFound
	}
	return content, nil
}

func (f *fakeNodeRepo) WriteReserved(_ context.Context, bundleAlias, relPath, content string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserved[bundleAlias+":"+relPath] = content
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// tempDirWithMD creates a temporary directory containing a single .md file
// and returns the directory path. Cleanup is handled by t.TempDir().
func tempDirWithMD(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "concept.md"), []byte("# Hello"), 0o644); err != nil {
		t.Fatalf("create test .md file: %v", err)
	}
	return dir
}

// seedConcept inserts a concept into repo under the given alias and relPath.
func seedConcept(t *testing.T, repo *fakeNodeRepo, alias, relPath, body string) {
	t.Helper()
	ref := domain.ConceptRef{BundleAlias: alias, RelativePath: relPath}
	concept := &domain.OKFConcept{
		Ref:         ref,
		Frontmatter: domain.OKFFrontmatter{Type: "note", Title: relPath},
		Body:        body,
	}
	if err := repo.Put(context.Background(), ref, concept); err != nil {
		t.Fatalf("seedConcept %s: %v", ref, err)
	}
}

// ---------------------------------------------------------------------------
// TestBundleService_AddBundle_FR001
// ---------------------------------------------------------------------------

// TestBundleService_AddBundle_FR001 validates FR-001 acceptance criteria:
//   - happy path registers the bundle and returns a populated entry
//   - duplicate alias returns ErrDuplicateAlias
//   - duplicate rootPath under a different alias returns ErrDuplicatePath
//   - non-existent rootPath returns ErrNotFound
//   - directory with no .md files returns ErrNotFound
func TestBundleService_AddBundle_FR001(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, br *fakeBundleRepo) (alias, rootPath string)
		wantErr   error
		wantAlias string
	}{
		{
			name: "happy path registers bundle and returns entry",
			setup: func(t *testing.T, _ *fakeBundleRepo) (string, string) {
				return "kb", tempDirWithMD(t)
			},
			wantAlias: "kb",
		},
		{
			name: "duplicate alias returns ErrDuplicateAlias",
			setup: func(t *testing.T, br *fakeBundleRepo) (string, string) {
				dir := tempDirWithMD(t)
				// Pre-register the alias under a different path so only the alias
				// check (not the path check) fires.
				_ = br.Put(context.Background(), domain.BundleEntry{
					Alias:    "kb",
					RootPath: "/some/other/path",
				})
				return "kb", dir
			},
			wantErr: domain.ErrDuplicateAlias,
		},
		{
			name: "duplicate rootPath under different alias returns ErrDuplicatePath",
			setup: func(t *testing.T, br *fakeBundleRepo) (string, string) {
				dir := tempDirWithMD(t)
				// Pre-register the same rootPath under a different alias.
				_ = br.Put(context.Background(), domain.BundleEntry{
					Alias:    "existing-kb",
					RootPath: dir,
				})
				return "new-kb", dir
			},
			wantErr: domain.ErrDuplicatePath,
		},
		{
			name: "non-existent rootPath returns ErrNotFound",
			setup: func(_ *testing.T, _ *fakeBundleRepo) (string, string) {
				return "kb", "/this/path/does/not/exist"
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name: "directory with no .md files returns ErrNotFound",
			setup: func(t *testing.T, _ *fakeBundleRepo) (string, string) {
				// t.TempDir() is empty — no .md files present.
				return "kb", t.TempDir()
			},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			br := newFakeBundleRepo()
			svc := &usecase.BundleService{
				BundleRepository: br,
				NodeRepository:   newFakeNodeRepo(),
			}

			alias, rootPath := tt.setup(t, br)
			got, err := svc.AddBundle(context.Background(), alias, rootPath, "desc", []string{"tag1"})

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("AddBundle error = %v, want errors.Is %v", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("AddBundle returned non-nil entry on error: %+v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("AddBundle unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("AddBundle returned nil entry on success")
			}
			if got.Alias != tt.wantAlias {
				t.Errorf("entry.Alias = %q, want %q", got.Alias, tt.wantAlias)
			}
			if got.RootPath != rootPath {
				t.Errorf("entry.RootPath = %q, want %q", got.RootPath, rootPath)
			}
			if got.CreatedAt.IsZero() {
				t.Error("entry.CreatedAt is zero")
			}

			// Verify the bundle was persisted in the repository.
			stored, getErr := br.Get(context.Background(), alias)
			if getErr != nil {
				t.Fatalf("bundle not persisted after AddBundle: %v", getErr)
			}
			if stored.Alias != alias {
				t.Errorf("persisted entry alias = %q, want %q", stored.Alias, alias)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBundleService_RemoveBundle_FR002
// ---------------------------------------------------------------------------

// TestBundleService_RemoveBundle_FR002 validates FR-002 acceptance criteria:
//   - happy path unregisters the bundle without touching the filesystem
//   - not-found alias returns a wrapped ErrNotFound
func TestBundleService_RemoveBundle_FR002(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, br *fakeBundleRepo) string // returns alias to remove
		wantErr error
	}{
		{
			name: "happy path removes registered bundle",
			setup: func(t *testing.T, br *fakeBundleRepo) string {
				_ = br.Put(context.Background(), domain.BundleEntry{
					Alias:    "kb",
					RootPath: tempDirWithMD(t),
				})
				return "kb"
			},
		},
		{
			name: "not-found alias returns ErrNotFound",
			setup: func(_ *testing.T, _ *fakeBundleRepo) string {
				return "nonexistent"
			},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			br := newFakeBundleRepo()
			svc := &usecase.BundleService{
				BundleRepository: br,
				NodeRepository:   newFakeNodeRepo(),
			}

			alias := tt.setup(t, br)
			err := svc.RemoveBundle(context.Background(), alias)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("RemoveBundle error = %v, want errors.Is %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("RemoveBundle unexpected error: %v", err)
			}

			// Confirm the bundle is gone from the repository.
			_, getErr := br.Get(context.Background(), alias)
			if !errors.Is(getErr, domain.ErrNotFound) {
				t.Errorf("bundle still present after RemoveBundle; Get error = %v", getErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBundleService_ListBundles_FR003
// ---------------------------------------------------------------------------

// TestBundleService_ListBundles_FR003 validates FR-003 acceptance criteria:
//   - empty registry returns a non-nil empty slice
//   - populated registry returns entries with correct Alias and ConceptCount
func TestBundleService_ListBundles_FR003(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setup        func(t *testing.T, br *fakeBundleRepo, nr *fakeNodeRepo)
		wantLen      int
		wantAlias    string
		wantConcepts int
	}{
		{
			name:    "empty registry returns empty non-nil slice",
			setup:   func(_ *testing.T, _ *fakeBundleRepo, _ *fakeNodeRepo) {},
			wantLen: 0,
		},
		{
			name: "populated registry returns entries with ConceptCount",
			setup: func(t *testing.T, br *fakeBundleRepo, nr *fakeNodeRepo) {
				_ = br.Put(context.Background(), domain.BundleEntry{
					Alias:    "kb",
					RootPath: "/some/path",
				})
				seedConcept(t, nr, "kb", "a.md", "body a")
				seedConcept(t, nr, "kb", "b.md", "body b")
			},
			wantLen:      1,
			wantAlias:    "kb",
			wantConcepts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			br := newFakeBundleRepo()
			nr := newFakeNodeRepo()
			tt.setup(t, br, nr)

			svc := &usecase.BundleService{
				BundleRepository: br,
				NodeRepository:   nr,
			}

			got, err := svc.ListBundles(context.Background())
			if err != nil {
				t.Fatalf("ListBundles unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("ListBundles returned nil slice")
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len(bundles) = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}

			var found *domain.BundleEntry
			for i := range got {
				if got[i].Alias == tt.wantAlias {
					found = &got[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("bundle alias %q not found in result", tt.wantAlias)
			}
			if found.ConceptCount != tt.wantConcepts {
				t.Errorf("ConceptCount = %d, want %d", found.ConceptCount, tt.wantConcepts)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBundleService_ReindexBundle_FR004
// ---------------------------------------------------------------------------

// TestBundleService_ReindexBundle_FR004 validates FR-004 acceptance criteria:
//   - happy path embeds all concepts, upserts them to the vector store, and
//     stamps LastIndexedAt on the persisted bundle entry
//   - unknown alias returns ErrNotFound
func TestBundleService_ReindexBundle_FR004(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setup          func(t *testing.T, br *fakeBundleRepo, nr *fakeNodeRepo)
		alias          string
		wantErr        error
		wantChunkCount int
	}{
		{
			name: "happy path re-embeds all concepts and updates LastIndexedAt",
			setup: func(t *testing.T, br *fakeBundleRepo, nr *fakeNodeRepo) {
				_ = br.Put(context.Background(), domain.BundleEntry{
					Alias:    "kb",
					RootPath: "/kb",
				})
				seedConcept(t, nr, "kb", "alpha.md", "alpha body text")
				seedConcept(t, nr, "kb", "beta.md", "beta body text")
			},
			alias:          "kb",
			wantChunkCount: 2,
		},
		{
			name:    "unknown alias returns ErrNotFound",
			setup:   func(_ *testing.T, _ *fakeBundleRepo, _ *fakeNodeRepo) {},
			alias:   "ghost",
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			br := newFakeBundleRepo()
			nr := newFakeNodeRepo()
			tt.setup(t, br, nr)

			vs := newFakeVectorStore()
			embedder := newFakeEmbedder(4)
			svc := &usecase.BundleService{
				BundleRepository: br,
				NodeRepository:   nr,
			}

			start := time.Now()
			err := svc.ReindexBundle(context.Background(), tt.alias, embedder, vs)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ReindexBundle error = %v, want errors.Is %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReindexBundle unexpected error: %v", err)
			}

			// Assert the vector store received the expected number of chunks.
			if got := vs.chunkCount(); got != tt.wantChunkCount {
				t.Errorf("vector store chunk count = %d, want %d", got, tt.wantChunkCount)
			}

			// Assert LastIndexedAt was updated on the persisted entry.
			entry, getErr := br.Get(context.Background(), tt.alias)
			if getErr != nil {
				t.Fatalf("Get bundle after ReindexBundle: %v", getErr)
			}
			if !entry.LastIndexedAt.After(start) {
				t.Errorf("LastIndexedAt = %v, want after %v", entry.LastIndexedAt, start)
			}
		})
	}
}
