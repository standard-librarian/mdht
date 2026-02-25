# ADR 0003: Phase 2 Internal Alpha Boundaries

## Status
Accepted

## Decision
Phase 2 focuses on local end-to-end study workflows:

- explicit single active session lifecycle (`start`/`end`)
- source-aware reader routing (`auto|markdown|pdf`) with external launch for video/course metadata flows
- cumulative progress updates (`before + delta`, clamped)
- CI coverage gate for `domain+usecase` >= 85%

## Deferred

- plugin RPC hardening and richer protocol surface
- collaboration transport/network synchronization

## Consequences

- internal alpha is locally usable and testable
- networked collaboration and plugin hardening move to next phase
