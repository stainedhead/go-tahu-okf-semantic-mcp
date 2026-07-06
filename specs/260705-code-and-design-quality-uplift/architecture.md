# Architecture: Code & Design Quality Uplift

**Feature:** code-and-design-quality-uplift
**Created:** 2026-07-05
**Status:** Draft

---

## Architecture Overview

This uplift does not introduce new external dependencies (beyond optionally `golang.org/x/sys` for flock) or new architectural layers. It hardens existing boundaries and eliminates guard asymmetries within the existing Clean Architecture.

The key structural addition is a **BundlePathResolver** in the adapter layer that becomes the single gateway for all filesystem path resolution. Every method in `FileNodeRepository` that previously built a path directly now delegates to the resolver.

```
cmd/tahu/main.go
    │
    ├── infra/config     ← validates all fields at load; accepts explicit path
    ├── infra/registry   ← flock around load→save
    ├── infra/transport  ← HTTP timeouts, non-loopback block, stdio ctx, panic recovery
    │
    └── adapter/
         ├── mcp/        ← consolidated ValidatePath (delegates to pathresolver)
         ├── okf/
         │    ├── BundlePathResolver  ← NEW: single validated-path gateway
         │    ├── FileNodeRepository  ← all path ops go through BundlePathResolver
         │    ├── validator.go        ← EvalSymlinks on target, not just root
         │    └── (indexer.go)        ← deleted in Phase 5
         ├── embedder/   ← zero-norm guard; IDF-based vocab selection
         └── vectorstore/← NaN guard in Search; dims+reset in Load

usecase/
    ├── ConceptService  ← WriteConcept adds use-case-layer path validation
    ├── BundleService   ← ReindexBundle scoped delete; clock injection
    └── SearchService   ← SemanticSearch/KeywordSearch distinguished

domain/
    ├── ConceptRef      ← NewConceptRef validating constructor
    ├── OKFFrontmatter  ← Validate() method
    ├── Clock           ← injectable time source
    └── domaintest/     ← NEW: shared fake implementations
```

---

## Component Architecture

### BundlePathResolver (FR-001)

The resolver holds the `alias→root` map and applies containment + symlink checks centrally. `FileNodeRepository` is refactored to own a `*BundlePathResolver` and call `Resolve` / `ResolveReserved` instead of building paths inline.

**Before:**
```
FileNodeRepository.Get → filepath.Join(root, relPath) → ValidateConceptPath → os.ReadFile
FileNodeRepository.ReadReserved → filepath.Join(root, filepath.Clean(relPath)) → os.ReadFile  ← NO CONTAINMENT
```

**After:**
```
FileNodeRepository.Get → resolver.Resolve(alias, relPath) → os.ReadFile
FileNodeRepository.ReadReserved → resolver.ResolveReserved(alias, relPath) → os.ReadFile
```

### NaN / zero-vector guard (FR-008/009)

Two-layer guard:
1. **Upsert-time:** `BM25Embedder.Embed` skips (or errors on) zero-norm outputs before they reach `HNSWStore.Upsert`.
2. **Search-time:** `HNSWStore.Search` sanitizes `NaN` scores to `-inf` (or filters them out) before sorting, as a defensive backstop.

### ReindexBundle scope-delete (FR-010)

Before upserting, `ReindexBundle` calls `store.Delete` with the IDs of all previously indexed chunks for the bundle. Chunk IDs follow the `alias:path:chunkIndex` format, so the set to delete is deterministic from the listed refs.

### flock registry (FR-028)

`YAMLBundleRepository` wraps its `load→save` sequence with `FileLock.Lock()` / `Unlock()`. The lock file is a companion to the registry YAML (e.g., `registry.yaml.lock`). The lock is held for the duration of the read-modify-write cycle only.

---

## Layer Responsibilities

| Layer | Owns | Does NOT own |
|---|---|---|
| `domain` | Invariant validation (constructors, Validate methods) | Filesystem, config, transport |
| `usecase` | Orchestration, business rules, clock injection | Filesystem ops, path resolution |
| `adapter/okf` | Path resolution, filesystem I/O, OKF parsing | Business rules, config |
| `adapter/mcp` | MCP protocol, input parsing, size limits | Business logic, filesystem |
| `infra/config` | Config loading, field validation, env overrides | Wiring, business logic |
| `infra/registry` | Bundle registry persistence, file locking | Business logic |
| `infra/transport` | HTTP/stdio lifecycle, timeouts, panic recovery | Tool implementations |
| `cmd/tahu` | Wiring, DI | Any logic whatsoever |

---

## Data Flow

### Concept read (post-uplift)

```
MCP handler
  → ValidatePath (mcp/validation.go — now delegates to pathresolver)
  → ConceptService.ReadConcept
  → FileNodeRepository.Get
  → BundlePathResolver.Resolve(alias, relPath)  ← containment + symlink check
  → os.ReadFile (capped with io.LimitReader)
  → ParseConcept
  → ExtractLinks
```

### Concept write (post-uplift)

```
MCP handler
  → ValidatePath + ValidateInputSize
  → ConceptService.WriteConcept
  → ref.Validate()                              ← use-case-layer check (FR-007)
  → fm.Validate()                               ← OKFFrontmatter.Validate() (FR-030)
  → bundleLock.Lock
  → FileNodeRepository.Put
  → BundlePathResolver.Resolve(alias, relPath)
  → os.WriteFile (atomic via mu)
  → regenerateIndex → WriteReserved → BundlePathResolver.ResolveReserved
  → appendLog    → WriteReserved → BundlePathResolver.ResolveReserved
  → bundleLock.Unlock
```

---

## Sequence Diagrams

_[TBD — add sequence diagrams for ReindexBundle scope-delete and flock registry during Phase 2–3 research]_

---

## Integration Points

- **flock:** Use `syscall.Flock` (stdlib, Unix-only). No new external dependency. Windows is deferred (NG8); gate the flock call behind a build tag (`//go:build !windows`) with a no-op stub for the Windows path.
- `coder/hnsw` — no API changes; NaN guard is in our layer
- `golangci-lint` — gosec linter added; all new code must be gosec-clean
- GitHub Actions — SHA-pinned; new integration test job added

---

## Architectural Decisions

_[TBD — record each significant decision as an ADR entry in `documents/arch-decisions-record.md` during implementation. Candidates: path resolver design, flock vs. single-writer constraint, zero-vector handling strategy, vocab IDF vs DF.]_
