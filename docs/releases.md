# Releases and Upgrades

This document defines the beta release format, verification steps, and upgrade procedure.

## Release artifacts

Each Git tag matching `v*` publishes:

- `mdht_<version>_linux_amd64.tar.gz`
- `mdht_<version>_linux_arm64.tar.gz`
- `mdht_<version>_darwin_amd64.tar.gz`
- `mdht_<version>_darwin_arm64.tar.gz`
- `checksums.txt`

Example tag:

- `v0.5.0-beta.1`

Then `<version>` in asset names is:

- `0.5.0-beta.1`

## Verify downloads

On macOS:

```bash
ASSET="mdht_0.5.0-beta.1_darwin_arm64.tar.gz"
grep " ${ASSET}$" checksums.txt | shasum -a 256 -c -
```

On Linux:

```bash
ASSET="mdht_0.5.0-beta.1_linux_amd64.tar.gz"
grep " ${ASSET}$" checksums.txt | sha256sum -c -
```

## Upgrade procedure

1. Download new asset and `checksums.txt`.
2. Verify checksum.
3. Stop running collab daemon(s):
   - `mdht collab daemon stop --vault <path>`
4. Replace binary.
5. Start daemon:
   - `mdht collab daemon start --vault <path>`
6. Validate:
   - `mdht collab status --vault <path>`
   - `mdht collab metrics --vault <path>`

## Phase 6 migration notes

On first v2 daemon startup, mdht auto-migrates legacy collab state in place.

- Trigger: presence of legacy `workspace.key` and missing `keys.json`.
- Backup location: `.mdht/collab/migrations/<timestamp>-v1-backup/`
- Migration marker: `.mdht/collab/migration.state`

Recommended post-upgrade checks:

1. `mdht collab daemon status --vault <path>`
2. `mdht collab status --vault <path>`
3. `mdht collab peer list --vault <path>`
4. `mdht collab sync now --vault <path>`

## Phase 7 connectivity notes

Phase 7 adds direct-peer NAT traversal attempts and connectivity diagnostics.

New commands:

1. `mdht collab net status --vault <path>`
2. `mdht collab net probe --vault <path>`
3. `mdht collab peer dial --peer-id <id> --vault <path>`
4. `mdht collab peer latency --vault <path> [--json]`
5. `mdht collab sync health --vault <path> [--json]`

Recommended post-upgrade checks:

1. `mdht collab daemon status --vault <path>`
2. `mdht collab net status --vault <path>`
3. `mdht collab net probe --vault <path>`
4. `mdht collab peer list --vault <path>`
5. `mdht collab sync health --vault <path>`

## Compatibility policy in beta

- CLI and docs may evolve between beta tags.
- Existing commands are kept stable where practical.
- For contract-level changes, ADRs under `docs/adr/` are updated.
