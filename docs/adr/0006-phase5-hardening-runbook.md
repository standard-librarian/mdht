# ADR 0006: Phase 5 Stabilization and Operator Runbook

## Status
Accepted

## Context
Phase 4 introduced collaboration transport and daemon control paths. The next requirement is beta hardening: deterministic CI, daemon lifecycle reliability, transport resilience, and operator-grade procedures.

## Decision
Phase 5 prioritizes non-functional hardening and documentation over feature expansion.

### Reliability decisions

1. Keep existing collab CLI/IPC contracts backward-compatible.
2. Add operational commands:
3. `mdht collab doctor`
4. `mdht collab daemon logs --tail <n>`
5. Enforce idempotent daemon start/stop/status behavior with stale pid/socket cleanup.
6. Add reconnect/backoff behavior and validation counters for transport troubleshooting.

### CI/toolchain decisions

1. Pin CI Go toolchain to `1.25.0`.
2. Require `go mod tidy`, clean `go.mod/go.sum` diff, and `go mod verify`.
3. Keep race tests, architecture boundary tests, and domain/usecase coverage gate blocking.
4. Run CI on Linux and macOS.

### Documentation decisions

1. Add an operator runbook for single-node and two-peer operations.
2. Document incident playbooks for stale socket/pid, key mismatch, split-brain suspicion, and replay recovery.

## Compatibility policy

1. CLI additions are additive only.
2. JSON-RPC method names are unchanged.
3. Status responses may add optional fields (counters) without removing existing fields.

## Consequences

1. Public beta collaboration is operable and diagnosable by humans without reading source code.
2. CI failures surface dependency drift early.
3. Transport/auth failures become observable via counters and doctor checks.
4. Phase 5 does not deliver new end-user features by design.
