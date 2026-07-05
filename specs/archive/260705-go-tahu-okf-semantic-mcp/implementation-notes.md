# Implementation Notes: go-tahu-okf-semantic-mcp

**Feature:** go-tahu-okf-semantic-mcp  
**Date:** 2026-07-05

Update this file as implementation progresses. Record decisions, surprises, gotchas, and deviations from the plan — not what the code does, but *why* it was done that way.

---

## Technical Decisions

### BM25 as sole embedding tier (deliberate deviation from spec)

The spec listed `internal/adapter/embedder/onnx.go` as a file to create. This was intentionally deferred. The ONNX MiniLM-L6-v2 tier requires CGo + libonnxruntime, which adds significant build complexity for v0.1. BM25 fully satisfies the zero-external-service guarantee and is the spec's "zero-CGo floor". The domain.Embedder interface is stable — ONNX can be added as a drop-in later.

**Why accepted**: BM25 meets all acceptance criteria G3, G5, G8. The ONNX tier is an enhancement, not a correctness requirement. Matches NFR "BM25 tier binary: go build only; no CGo".

### pkg/okfcodec/ not implemented (deliberate deviation from spec)

The spec listed `pkg/okfcodec/` as a utility package to export. All codec logic lives in `internal/adapter/okf/` where it is well-tested. Exporting it to `pkg/` would duplicate code and create a second maintenance surface without a current consumer. This can be extracted when an external consumer exists.

**Why accepted**: No external consumer in v0.1. Internal use is fully served by the adapter package.

---

## Edge Cases & Solutions

_[Record edge cases encountered and how they were resolved]_

---

## Deviations from Plan

_[Record any deviations from plan.md or spec.md with rationale]_

---

## Lessons Learned

_[Record lessons for future specs — to be written during Phase 6 archival]_
