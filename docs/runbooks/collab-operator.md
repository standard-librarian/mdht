# Collaboration Operator Runbook (Phase 7)

This runbook covers single-vault daemon operations, two-peer bootstrap, health checks, and incident recovery for `mdht` collaboration transport.

## Scope and ownership

- Supported platforms: macOS + Linux.
- Sync scope: mdht-managed content only (`source/*`, `session/*`, `topic/*`).
- Unmanaged markdown is not replicated by collab and must remain unchanged.

## Prerequisites

- Same `mdht` version on all peers.
- Direct peer connectivity path; NAT traversal is attempted automatically.
- No relay fallback in this phase.
- Workspace key trust model accepted by operators.

## Single-node setup

1. Initialize workspace identity and key material:

```bash
mdht collab workspace init --name "team-alpha" --vault /path/to/vault
```

2. Start daemon:

```bash
mdht collab daemon start --vault /path/to/vault
```

3. Verify health:

```bash
mdht collab daemon status --vault /path/to/vault
mdht collab status --vault /path/to/vault
mdht collab net status --vault /path/to/vault
mdht collab net probe --vault /path/to/vault
mdht collab sync health --vault /path/to/vault
mdht collab metrics --vault /path/to/vault
```

4. Inspect runtime logs:

```bash
mdht collab daemon logs --tail 200 --vault /path/to/vault
```

## Two-peer bootstrap

### Node A

1. Start daemon and read listen addresses:

```bash
mdht collab daemon start --vault /vault/A
mdht collab daemon status --vault /vault/A
```

2. Copy a reachable `/ip4/.../tcp/.../p2p/<peer-id>` address from status output.

### Node B

1. Initialize workspace with matching trust context:

```bash
mdht collab workspace init --name "team-alpha" --vault /vault/B
```

2. Add Node A as bootstrap peer:

```bash
mdht collab peer add --addr /ip4/<A-IP>/tcp/<A-PORT>/p2p/<A-PEER-ID> --vault /vault/B
mdht collab peer approve --peer-id <A-PEER-ID> --vault /vault/B
```

3. Start daemon and validate:

```bash
mdht collab daemon start --vault /vault/B
mdht collab status --vault /vault/B
mdht collab metrics --vault /vault/B
```

4. Optionally add B->A reciprocal peer entry:

```bash
mdht collab peer add --addr /ip4/<B-IP>/tcp/<B-PORT>/p2p/<B-PEER-ID> --vault /vault/A
mdht collab peer approve --peer-id <B-PEER-ID> --vault /vault/A
```

To permanently remove a peer entry (as opposed to revoking it):

```bash
mdht collab peer remove --peer-id <id> --vault <path>
```

## Health checks

Run these checks in order when debugging replication:

1. `mdht collab workspace show --vault <path>`
2. `mdht collab daemon status --vault <path>`
3. `mdht collab status --vault <path>`
4. `mdht collab peer list --vault <path>`
5. `mdht collab daemon logs --tail 300 --vault <path>`
6. `mdht collab net status --vault <path>`
7. `mdht collab net probe --vault <path>`
8. `mdht collab sync health --vault <path>`
9. `mdht collab metrics --vault <path>`
10. `mdht collab activity tail --limit 100 --vault <path>`
11. `mdht collab presence --vault <path>`

Interpretation:

- `daemon status`: process/socket health.
- `status`: runtime online/peer/pending ops and counters.
- `net status/probe`: traversal mode, reachability, dialable addresses.
- `sync health`: high-level convergence state (`good|degraded|down`).
- `metrics`: realtime counters and queue depths.
- log tail: authentication, decode, and reconnect details.
- `presence`: per-peer reachability, RTT, dial result, and traversal mode summary.

Use `mdht collab presence tail --vault <path>` to stream presence-related events only. Use `mdht collab peer timeline --peer-id <id> --vault <path>` for presence history filtered to a specific peer.

## Incident playbooks

### Stale socket or stale PID

Symptoms:

- `daemon start` fails with socket/pid conflict.
- `daemon status` reports socket unreachable.

Actions:

1. Stop daemon idempotently:

```bash
mdht collab daemon stop --vault <path>
```

2. Start again:

```bash
mdht collab daemon start --vault <path>
```

3. Verify with `daemon status` and `metrics`.

### Key mismatch/auth failures

Symptoms:

- `peer add` fails with auth or workspace mismatch.
- counters show `invalid_auth` / `workspace_mismatch` increasing.

Actions:

1. Verify peers are in intended workspace.
2. Re-create workspace material only if organizationally approved.
3. Re-add peer addresses and verify counters stop increasing.

### NAT traversal or dial failures

Symptoms:

- `peer dial` returns failure repeatedly.
- `net status` remains degraded/down.

Actions:

1. Run active probe:

```bash
mdht collab net probe --vault <path>
```

2. Force peer dial and latency checks:

```bash
mdht collab peer dial --peer-id <id> --vault <path>
mdht collab peer latency --vault <path>
```

3. Confirm both peers approve each other and share workspace key lineage.
4. If failure persists, collect `daemon logs` + `activity tail` and retry after endpoint/IP change.

### Split-brain suspicion

Symptoms:

- Divergent managed block content between peers.

Actions:

1. Trigger sync on both peers:

```bash
mdht collab sync now --vault <path>
```

2. Compare exported state snapshots:

```bash
mdht collab snapshot export --out /tmp/collab-snapshot.json --vault <path>
```

3. If needed, run `mdht reindex` after convergence validation.

### Replay/recovery from local state

1. Stop daemon.
2. Ensure vault managed files are intact.
3. Restart daemon.
4. Run `mdht collab sync now`.
5. Validate with `status`, `metrics`, and sample note checks.

## Upgrade and restart procedure

1. Stop daemon on each node.
2. Upgrade binary.
3. Start daemon.
4. Run `status` and `metrics` checks.
5. Trigger one manual sync after full rollout.

## Exit criteria (operations)

A node is considered healthy when:

- `daemon status` is running with valid socket.
- `status` shows expected peer count and stable counters.
- replication converges after manual sync.
