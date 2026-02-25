# ADR 0004: Plugin SDK v1 Contract Freeze

## Status
Accepted

## Decision
Phase 3 freezes the plugin surface for beta readiness across three boundaries:

1. CLI contract (`plugin list/doctor/commands/exec/analyze/tty`)
2. Manifest contract (`plugins/plugins.json` with strict capability and checksum validation)
3. RPC contract (`MdhtPlugin` with `GetMetadata`, `ListCommands`, `Execute`, `PrepareTTY`)

The runtime remains local-first and checksum allowlist based.

## Compatibility Policy

1. RPC method set is fixed for v1.
2. Existing fields are never removed or renumbered.
3. New fields are additive only.
4. CLI command names/flags are backward compatible for v1.

## Deferred

1. Plugin marketplace/distribution.
2. Remote execution/sandboxing hardening.
3. Collaboration transport networking.
