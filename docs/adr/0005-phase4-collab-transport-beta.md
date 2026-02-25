# ADR 0005: Phase 4 Collaboration Transport (Public Beta)

## Status
Accepted

## Context
Phase 2 and 3 left collaboration networking deferred. `collab` only exposed local `export-state` with no daemon, no peer transport, and no runtime visibility.

Phase 4 requires:
- daemonized per-vault collaboration runtime
- peer-to-peer replication using manual bootstrap peers
- shared workspace-key authentication
- pure-Go CRDT convergence for mdht-managed state
- stable local IPC contract for CLI/TUI control

## Decision
We implement collaboration transport as a dedicated daemon and freeze a local JSON-RPC control contract.

### Transport
- Use `go-libp2p` for direct peer connections.
- Manual bootstrap peers are configured in `<vault>/.mdht/collab/peers.json`.
- Relay/NAT traversal is explicitly deferred.

### Auth model
- Workspace key: `<vault>/.mdht/collab/workspace.key`.
- Node identity key: `<vault>/.mdht/collab/node.ed25519`.
- Peers must pass workspace challenge/response HMAC before op/sync streams are accepted.
- Operation envelopes are HMAC-signed and validated before merge.

### CRDT model
- OR-Map keyed by `source/*`, `session/*`, `topic/*`.
- Registers for scalar fields, OR-Set for set fields, sequence CRDT for managed text blocks.
- Tombstone-based snapshot refresh to support deterministic convergence and removals.

### Scope and ownership
- Collab mutates only mdht-owned frontmatter/managed blocks for source/session/topic notes.
- Unmanaged markdown is preserved.
- Vault markdown remains source-of-truth.

### IPC contract
- Unix socket: `<vault>/.mdht/collab/daemon.sock`.
- JSON-RPC methods: `WorkspaceInit`, `WorkspaceShow`, `PeerAdd`, `PeerRemove`, `PeerList`, `Status`, `ReconcileNow`, `ExportState`, `Stop`.

## Consequences
- Collaboration can run continuously and reconcile across peers.
- CLI and TUI gain operational visibility and manual reconcile control.
- Contract surfaces are explicit for beta hardening and future compatibility.
- Marketplace/distribution and sandboxing stay out of scope.
