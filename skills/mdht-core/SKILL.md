---
name: mdht-core
description: Build and maintain mdht using strict hexagonal architecture per module (adapter-in to usecase to service to port-out to adapter-out). Use when adding commands, module features, vault markdown handling, SQLite projections, plugin runtime checks, TUI updates, or architecture/test enforcement in this repository.
---

# mdht Core

## Core workflow

1. Identify the target module (`library`, `reader`, `session`, `graph`, `plugin`, `collab`).
2. Implement changes inside that module layers before touching cross-module wiring.
3. Keep `usecase` above `service`; keep adapters isolated from business rules.
4. Add or update outbound ports before implementing adapter/out behavior.
5. Wire inbound adapters through module `port/in` interfaces only.
6. Add tests for behavior and architecture boundaries.

## Hard constraints

- Keep cross-module interaction through `port/in` interfaces.
- Avoid importing adapter packages from `usecase`, `service`, or `domain`.
- Keep vault markdown as source-of-truth; SQLite is projection.
- Preserve non-managed markdown content; mutate only mdht-managed block and owned frontmatter keys.
- Use Catppuccin Mocha token styles from `internal/ui/theme`.

## Command checklist

- Format and test with:
  - `go test ./...`
  - `go test ./... -race`
- Validate architecture boundaries with:
  - `go test ./internal/architecture -run TestHexagonalLayerImports`
- Validate skills with:
  - `scripts/validate-skills.sh`

## References

- Architecture map: `references/architecture.md`
- CLI and quality conventions: `references/commands.md`
- Repeated verification command: `scripts/run_quality.sh`
