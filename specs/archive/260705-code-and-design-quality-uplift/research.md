# Research: Code & Design Quality Uplift

**Feature:** code-and-design-quality-uplift
**Created:** 2026-07-05
**Source PRD:** specs/260705-code-and-design-quality-uplift/code-and-design-quality-uplift-PRD.md

---

## Research Questions

1. **flock portability (FR-028):** Does `syscall.Flock` / `golang.org/x/sys/unix.Flock` work correctly on macOS and Linux for the registry YAML file? What happens if a process holding the lock crashes — is the lock automatically released? Are there any edge cases with NFS-mounted paths?

2. **NaN guard strategy (FR-008/009):** Should zero-norm vectors be rejected at `Upsert` time (return error), silently skipped (log + no-op), or normalized to a unit vector? What does `coder/hnsw` do with a NaN node internally — does it corrupt graph structure or just return NaN scores?

3. **BM25 vocab IDF selection (FR-012):** What is the standard approach for selecting a fixed-dimension sparse vocabulary by discriminativeness? Options: sort by IDF descending, sort by `tf*idf` product, or use a chi-square/mutual-information selector. What is the performance impact of sorting by IDF rather than DF given the current corpus sizes?

4. **`io.LimitReader` vs stat-and-reject (FR-024):** For the file read cap, is it better to stat the file and return an error before reading (user sees a meaningful error), or use `io.LimitReader` and silently truncate? Should the cap be configurable or hardcoded? What is the right cap given OKF documents are spec'd at ≤ 1 MB in the MCP write path?

5. **`filepath.EvalSymlinks` on non-existent paths (FR-006):** ✅ Resolved. Use `EvalSymlinks` for existing paths; use `os.Lstat` on the final path component for new paths — if `Lstat` returns no error and the entry is a symlink, reject. If `Lstat` returns `os.ErrNotExist`, path is new and safe. Never call `EvalSymlinks` on a path that may not exist.

---

## Industry Standards

_[TBD — populate during Phase 2 research]_

---

## Existing Implementations

_[TBD — review coder/hnsw source for NaN handling; review golang.org/x/sys/unix for flock API]_

---

## API Documentation

_[TBD — coder/hnsw v0.6.1 CosineDistance; golang.org/x/sys/unix Flock constants]_

---

## Best Practices

_[TBD]_

---

## Open Questions

_All PRD open questions resolved. See status.md. Research questions above are implementation-level unknowns to resolve during Phase 1–2._

---

## References

- coder/hnsw v0.6.1: https://github.com/coder/hnsw
- Go flock: https://pkg.go.dev/golang.org/x/sys/unix#Flock
- BM25 Wikipedia: https://en.wikipedia.org/wiki/Okapi_BM25
