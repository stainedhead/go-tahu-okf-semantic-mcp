// Package domain defines the core OKF knowledge-management types, interfaces,
// and sentinel errors. It has zero external dependencies (stdlib only).
package domain

import "time"

// BundleEntry holds registration metadata for a single OKF bundle.
type BundleEntry struct {
	Alias         string // user-assigned unique identifier
	RootPath      string // absolute, canonicalized filesystem path
	Description   string
	Tags          []string
	CreatedAt     time.Time
	LastIndexedAt time.Time
	// ChunkIDs holds the IDs of all vector-store chunks indexed during the
	// last ReindexBundle run. It is persisted so that the next reindex can
	// delete stale chunks (concepts that were removed since the last run).
	ChunkIDs []string
	// ConceptCount is a derived field populated by the use case layer via
	// NodeRepository.List; it is not persisted in the registry store.
	ConceptCount int
}
