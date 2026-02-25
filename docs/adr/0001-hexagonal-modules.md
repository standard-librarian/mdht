# ADR 0001: Hexagonal Modules and SOLID Boundaries

## Status
Accepted

## Context
mdht needs strict modularity to keep code clean while supporting vault IO, SQLite projections, TUI, plugin runtime, and collaboration scaffolding.

## Decision
- Enforce module-local hexagonal layers:
  - `adapter/in`
  - `usecase`
  - `service`
  - `domain`
  - `port/in`
  - `port/out`
  - `adapter/out`
  - `dto`
- Require cross-module interactions to happen through `port/in` interfaces.
- Keep `usecase` above `service` in every module.
- Keep adapters isolated from business logic.

## Consequences
- Higher upfront boilerplate but lower long-term coupling.
- Easier testability and replacement of external dependencies.
- Automated architecture tests can enforce boundaries.
