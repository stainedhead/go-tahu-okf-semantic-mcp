# Research: Code & Design Quality Uplift — Auto-Review Fixes

**Feature:** code-and-design-quality-uplift-auto-review
**Created:** 2026-07-06
**Source PRD:** specs/260706-code-and-design-quality-uplift-auto-review/code-and-design-quality-uplift-auto-review-PRD.md

---

## Research Questions

1. **adapter/mcp coverage gap**: Current coverage is 61.4% against a 75% spec target. Which handler functions in `internal/adapter/mcp/handlers.go` are uncovered? What is the realistic achievable floor given the existing test infrastructure (handlers_test.go uses real repos)?

2. **domaintest migration scope**: Which tests in `internal/usecase/` use private fakes that duplicate domaintest? After fixing the interface, what is the minimal change to migrate one consumer without breaking the rest of the test suite?

3. **HNSWStore.Delete correctness**: Does `s.graph.Delete(id)` from `coder/hnsw` actually remove the node from search results, or does the HNSW graph require a rebuild? Need to verify the library's delete semantics before writing the test.

4. **ValidateConceptPath callers**: Are there any callers of `ValidateConceptPath` outside `repository_coverage_test.go`? Run `grep -rn ValidateConceptPath` across all non-test files to confirm zero production callers before unexport/deletion.

5. **adapter/vectorstore gap**: Current 76.4% vs 85% target. Which untested paths exist? `Delete` is 0% — does adding `TestHNSWStore_Delete` alone close enough of the gap?

---

## Findings

_To be populated during implementation._

---

## References

- `internal/adapter/mcp/handlers_test.go` — existing test patterns
- `internal/domain/domaintest/fakes.go` — fake implementations to fix
- `internal/adapter/vectorstore/hnsw.go` — Delete method at line 181
- `internal/adapter/okf/validator.go` — ValidateConceptPath at line 22
