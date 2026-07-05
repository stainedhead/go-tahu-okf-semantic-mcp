# specs/

Feature specification and tracking for all planned and in-progress work.

## Naming convention

Each feature gets a directory named `YYMMDD-<feature-name>/` where `YYMMDD` is the spec *creation* date (not the implementation date).

## Per-spec directory contents

| File | Purpose |
|---|---|
| `spec.md` | Feature specification and requirements (populated from PRD) |
| `status.md` | **CRITICAL** — phase progress tracking; update after every task |
| `plan.md` | Implementation plan and architecture decisions |
| `tasks.md` | Task breakdown with dependencies and estimates |
| `research.md` | Research findings, API docs, examples |
| `data-dictionary.md` | Data structures, types, schemas |
| `architecture.md` | System architecture and component design |
| `implementation-notes.md` | Decisions, gotchas, edge cases |
| `<feature>-PRD.md` | Source PRD (moved here by `create-spec`) |

## Lifecycle

Active specs live in `specs/`. When `status.md` reaches 100% completion, run `/archive-spec <dir>` to move the spec to `specs/archive/`.

## Note on `spec/` vs `specs/`

`spec/` (no 's') at the repo root holds the SDD source-of-truth specs by domain: `spec/tools/`, `spec/domain/`, `spec/usecases/`. These are stable interface contracts referenced by tests.

`specs/` (this directory) holds the *feature development* lifecycle artifacts — the living planning and tracking documents for in-flight and completed features.
