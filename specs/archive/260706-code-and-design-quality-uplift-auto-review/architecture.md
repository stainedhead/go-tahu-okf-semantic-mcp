# Architecture: Code & Design Quality Uplift — Auto-Review Fixes

**Feature:** code-and-design-quality-uplift-auto-review
**Created:** 2026-07-06
**Status:** Draft

No architectural changes. All fixes are corrections within existing packages. The dependency graph (domain ← usecase ← adapter ← infra) is unchanged.

---

## Component Changes

| Component | Change |
|---|---|
| `internal/domain/domaintest/fakes.go` | Fix VectorStore.Delete method signature |
| `internal/usecase/*_test.go` | Import domaintest instead of private fakes |
| `internal/adapter/vectorstore/hnsw_test.go` | Add Delete test |
| `.github/workflows/ci.yml` | Add two coverage gate steps |
| `internal/adapter/okf/validator.go` | Unexport ValidateConceptPath |
