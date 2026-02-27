# Collaboration Public Beta Guide

This guide covers the supported collaboration flow for public beta users.

For deep operational incidents, see the [operator runbook](runbooks/collab-operator.md).

## Beta model

- Topology: peer-to-peer, manual bootstrap peers.
- Trust: shared workspace key + node identity.
- Transport: direct peers with NAT traversal attempts, no relay fallback.
- Sync scope: mdht-managed content only.

Not in scope for this beta:

- full-vault unmanaged markdown sync
- relay infrastructure
- plugin marketplace/distribution

## Single-node setup

```bash
VAULT=/path/to/vault
mdht collab workspace init --name "team-alpha" --vault "$VAULT"
mdht collab daemon start --vault "$VAULT"
mdht collab daemon status --vault "$VAULT"
mdht collab status --vault "$VAULT"
mdht collab net status --vault "$VAULT"
mdht collab net probe --vault "$VAULT"
mdht collab sync health --vault "$VAULT"
mdht collab metrics --vault "$VAULT"
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
mdht collab peer approve --peer-id <A-PEER-ID> --vault /vault/B
mdht collab daemon start --vault /vault/B
mdht collab status --vault /vault/B
```

Optional reciprocal entry:

```bash
mdht collab peer add --addr /ip4/<B-IP>/tcp/<B-PORT>/p2p/<B-PEER-ID> --label "node-b" --vault /vault/A
mdht collab peer approve --peer-id <B-PEER-ID> --vault /vault/A
```

## Health checks

Run in this order:

1. `mdht collab workspace show --vault <path>`
2. `mdht collab daemon status --vault <path>`
3. `mdht collab status --vault <path>`
4. `mdht collab peer list --vault <path>`
5. `mdht collab net status --vault <path>`
6. `mdht collab net probe --vault <path>`
7. `mdht collab sync health --vault <path>`
8. `mdht collab daemon logs --tail 200 --vault <path>`
9. `mdht collab metrics --vault <path>`
10. `mdht collab activity tail --limit 50 --vault <path>`

Key status counters:

- `auth_invalid`
- `auth_workspace_mismatch`
- `auth_unauthenticated_peer`
- `transport_decode_errors`
- `transport_reconnect_attempts`
- `transport_reconnect_successes`
- `transport_dial_attempts`
- `transport_dial_successes`
- `transport_dial_failures`
- `transport_hole_punch_attempts`
- `transport_hole_punch_successes`

## Safety boundary: unmanaged content

Collab must not overwrite unmanaged markdown text. If you suspect this happened:

1. Stop daemon: `mdht collab daemon stop --vault <path>`
2. Export state: `mdht collab snapshot export --out /tmp/collab-snapshot.json --vault <path>`
3. Run manual sync after inspection: `mdht collab sync now --vault <path>`
4. Use the operator runbook incident section for recovery steps.
