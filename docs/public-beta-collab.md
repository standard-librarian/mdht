# Collaboration Public Beta Guide

This guide covers the supported collaboration flow for public beta users.

For deep operational incidents, see the [operator runbook](runbooks/collab-operator.md).

## Beta model

- Topology: peer-to-peer, manual bootstrap peers.
- Trust: shared workspace key + node identity.
- Transport: direct routable peers (no relay/NAT traversal in this phase).
- Sync scope: mdht-managed content only.

Not in scope for this beta:

- full-vault unmanaged markdown sync
- relay infrastructure or NAT traversal
- plugin marketplace/distribution

## Single-node setup

```bash
VAULT=/path/to/vault
mdht collab workspace init --name "team-alpha" --vault "$VAULT"
mdht collab daemon start --vault "$VAULT"
mdht collab daemon status --vault "$VAULT"
mdht collab doctor --vault "$VAULT"
```

## Two-peer bootstrap

### On Node A

```bash
mdht collab daemon start --vault /vault/A
mdht collab daemon status --vault /vault/A
```

Copy one reachable `/ip4/.../tcp/.../p2p/<peer-id>` address.

### On Node B

```bash
mdht collab workspace init --name "team-alpha" --vault /vault/B
mdht collab peer add --addr /ip4/<A-IP>/tcp/<A-PORT>/p2p/<A-PEER-ID> --vault /vault/B
mdht collab daemon start --vault /vault/B
mdht collab status --vault /vault/B
```

Optional reciprocal entry:

```bash
mdht collab peer add --addr /ip4/<B-IP>/tcp/<B-PORT>/p2p/<B-PEER-ID> --vault /vault/A
```

## Health checks

Run in this order:

1. `mdht collab doctor --vault <path>`
2. `mdht collab daemon status --vault <path>`
3. `mdht collab status --vault <path>`
4. `mdht collab peer list --vault <path>`
5. `mdht collab daemon logs --tail 200 --vault <path>`

Key status counters:

- `invalid_auth`
- `workspace_mismatch`
- `unauthenticated`
- `decode_errors`
- `reconnect_attempts`
- `reconnect_successes`

## Safety boundary: unmanaged content

Collab must not overwrite unmanaged markdown text. If you suspect this happened:

1. Stop daemon: `mdht collab daemon stop --vault <path>`
2. Export state: `mdht collab export-state --vault <path>`
3. Reconcile manually after inspection: `mdht collab reconcile --vault <path>`
4. Use the operator runbook incident section for recovery steps.

