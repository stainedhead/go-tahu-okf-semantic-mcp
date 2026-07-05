# GitHub Copilot Instructions

@../AGENTS.md

---

## Copilot-specific notes

- Complete functions using the Clean Architecture layer of the file being edited — do not cross layer boundaries in suggestions.
- When suggesting a new function, check whether a domain interface already captures the contract before adding a concrete type.
- Prefer table-driven test completions with subtests named after spec requirements.
- Do not suggest global variables or `init()` functions.
- Do not suggest `panic()` outside of `main` or truly unrecoverable startup conditions.
