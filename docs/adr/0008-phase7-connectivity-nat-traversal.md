# ADR 0008: Phase 7 Connectivity Expansion (Direct + NAT Traversal, No Relay)

## Status
Accepted

## Context

Phase 6 delivered collaboration v2 contracts, peer allowlist, key rotation, activity/conflict surfaces, and migration.
Connectivity still depended on directly routable peers, which limited real-world multi-network beta adoption.

Phase 7 requires:

1. stronger connectivity across common NAT environments,
2. no relay dependency,
3. operational diagnostics for connectivity and sync health,
4. continued managed-content-only sync boundaries.

## Decision

### Connectivity model

Use direct peer dialing with NAT traversal attempts enabled in libp2p host configuration.
Relay remains out of scope for this phase.

Implemented runtime behavior:

1. dial ladder: direct attempt -> traversal attempt -> bounded retry,
2. dial outcomes persisted into peer metadata (`last_dial_result`, `rtt_ms`, `traversal_mode`, `reachability`),
3. counters for dial and traversal attempts/successes/failures.

### New public surfaces

Added CLI/IPC capabilities:

1. `collab net status`
2. `collab net probe`
3. `collab peer dial`
4. `collab peer latency`
5. `collab sync health`

Status payloads now include:

1. reachability,
2. NAT mode,
3. connectivity health (`good|degraded|down`).

### Data contract updates

`peers.json` extends with connectivity health fields:

1. `reachability`
2. `last_dial_at`
3. `last_dial_result`
4. `rtt_ms`
5. `traversal_mode`

`activity.log` now includes connectivity lifecycle event types (`dial_*`, `hole_punch_*`).

## Consequences

1. Operators can diagnose and recover connectivity failures without reading code.
2. Beta users gain higher chance of successful peer connectivity without relay infrastructure.
3. CLI/IPC status shapes are expanded; automation consumers should parse fields defensively.
4. Managed markdown ownership boundaries remain unchanged.
