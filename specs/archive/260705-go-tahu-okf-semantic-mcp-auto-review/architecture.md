# Architecture: go-tahu-okf-semantic-mcp-auto-review

**Feature:** go-tahu-okf-semantic-mcp-auto-review  
**Date:** 2026-07-05  
**Status:** Draft

---

## Architecture Overview

These are bug fixes and compliance patches — no new architectural layers or packages are introduced. The changes tighten the existing Clean Architecture by moving responsibilities to their correct layers.

---

## Component Architecture

### FR-001: Binary removal

No code change. `.gitignore` update + `git rm --cached tahu`.

### FR-002: Remove index/log side effects from `FileNodeRepository.Put`

**Current (broken) state:**
```
HandleConceptWrite
  → ConceptService.WriteConcept
      → FileNodeRepository.Put           ← writes concept + index.md + log.md (1st write)
      → ConceptService.regenerateIndex   ← overwrites index.md (2nd write, lower quality)
      → ConceptService.appendLog         ← appends log.md (2nd entry)
```

**Target state:**
```
HandleConceptWrite
  → ConceptService.WriteConcept
      → FileNodeRepository.Put           ← writes concept ONLY
      → ConceptService.regenerateIndex   ← writes index.md (single write, improved quality)
      → ConceptService.appendLog         ← appends log.md (single entry)
```

### FR-003: Improved `regenerateIndex`

`ConceptService.regenerateIndex` calls `NodeRepository.List` → for each ref, calls `NodeRepository.Get` → builds `| File | Type | Title |` table. Get failures are tolerated (empty values, no abort).

### FR-004: Bundle-scoped advisory mutex in `ConceptService`

`ConceptService` gains a `bundleMu sync.Map` field. `WriteConcept` acquires a per-bundle `sync.Mutex` before calling Put, regenerateIndex, and appendLog, releasing it after:

```
bundleMu.LoadOrStore(alias, &sync.Mutex{})
mu.Lock()
defer mu.Unlock()
// ... Put, regenerateIndex, appendLog
```

This serializes the full write sequence per bundle while allowing writes to different bundles to proceed in parallel. The existing `FileNodeRepository.mu` continues to protect the internal repository state.

### FR-005: request_id context propagation

`loggingMiddleware` in `server.go`:
1. Generates `requestID := uuid.New().String()`
2. Stores it: `ctx = context.WithValue(ctx, requestIDKey, requestID)`
3. Logs it as before

New exported helper: `RequestIDFromContext(ctx context.Context) string` in `internal/infra/transport/`.

### FR-006: Alias validation

`HandleBundleAdd` in `handlers.go`: before calling `BundleService.AddBundle`, checks `strings.Contains(alias, ":")` and returns an error if true.

---

## Layer Responsibilities After Fixes

| Layer | Responsibility |
|---|---|
| `adapter/okf` | Persist concept documents (Put). No index/log side effects. |
| `usecase` | Orchestrate Put + regenerateIndex + appendLog under advisory mutex. |
| `adapter/mcp` | Input validation (alias format, path traversal, size). |
| `infra/transport` | Request ID generation and context propagation. |

---

## Data Flow — `concept_write` after fixes

```
MCP client
  → HandleConceptWrite (adapter/mcp)
      → validate input (path, size, type)
      → alias colon check (FR-006 addition for bundle_add, not concept_write)
      → ConceptService.WriteConcept (usecase)
          → acquire bundle advisory mutex (FR-004)
          → FileNodeRepository.Put → write concept.md ONLY
          → regenerateIndex → Get each ref → write index.md
          → appendLog → ReadReserved + WriteReserved → write log.md
          → release bundle advisory mutex
```

---

## Sequence Diagrams

### Concurrent writes after FR-002 + FR-004

```
Goroutine A (alias="kb", path="a.md")   Goroutine B (alias="kb", path="b.md")
acquire bundleMu["kb"]                  block on bundleMu["kb"]
Put("a.md")                             ...
regenerateIndex (list: a.md)            ...
appendLog ("a.md wrote")                ...
release bundleMu["kb"]                  acquire bundleMu["kb"]
                                        Put("b.md")
                                        regenerateIndex (list: a.md, b.md)
                                        appendLog ("b.md wrote")
                                        release bundleMu["kb"]
log.md: "a.md wrote" + "b.md wrote"    ← both entries preserved
```

---

## Integration Points

No changes to MCP tool schemas, CLI flags, config keys, or HTTP endpoints.

---

## Architectural Decisions

- **FR-004 approach: bundle-scoped advisory mutex over adapter-side locking.** Keeps the use case layer owning index/log lifecycle (Clean Architecture). Does not require interface changes. Acceptable latency impact for v0.1 bundle sizes.
- **FR-003: N+1 Get calls in regenerateIndex.** Accepted for v0.1. Bundles are small; optimize with batch in v0.2 if profiling shows it matters.
- **FR-005: unexported contextKey type.** Prevents accidental key collisions from other packages using `context.WithValue("request_id", ...)`.
