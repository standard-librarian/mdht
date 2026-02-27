# ADR 0007: Phase 6 Collaboration v2 Hard Cut

## Status
Accepted

## Context

Phase 4/5 collaboration was public beta with v1-style command and IPC surfaces (`reconcile`, `export-state`, `doctor`) and shared-key auth without peer admission policy or key rotation workflow.

Phase 6 requires:

1. v2 hard cut for collaboration contracts.
2. per-peer allowlist states (`pending|approved|revoked`).
3. workspace key rotation with grace support.
4. activity/conflict surfaces for operators and TUI.
5. automatic in-place migration from existing v1 state.

## Decision

### Contract hard cut

We replace v1 collab command and IPC names with v2-only surfaces.

CLI highlights:

- `collab workspace rotate-key`
- `collab peer approve|revoke`
- `collab activity tail`
- `collab conflicts list|resolve`
- `collab sync now`
- `collab snapshot export`
- `collab metrics`

Removed:

- `collab reconcile`
- `collab export-state`
- `collab doctor`

IPC namespace is `CollabV2.*` only.

### Data contract updates

`OpEnvelope` now includes:

- `workspace_key_id`
- `peer_id`
- `schema_version=2`

Workspace/keys/peers store contract updated for v2:

- `workspace.json` includes `workspace_id` and `schema_version`.
- `keys.json` tracks `active_key_id`, key set, and rotation grace horizon.
- `peers.json` tracks peer policy state and metadata.

### Migration

On daemon startup, if legacy `workspace.key` exists and `keys.json` does not:

1. create timestamped backup under `.mdht/collab/migrations/`.
2. migrate workspace/keys/peers/oplog to v2 shape.
3. persist `migration.state`.
4. fail with typed migration error on any partial/invalid conversion.

## Consequences

1. Collaboration v2 is explicit and operation-focused for beta users.
2. Peer admission and key rotation improve trust controls.
3. Existing automation/scripts must update to v2 command names.
4. Migration is automatic but guarded by backup and failure signaling.

