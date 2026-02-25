# Getting Started

This guide gets a new beta tester from install to a full local study loop.

## Prerequisites

- macOS or Linux
- an Obsidian vault path
- `mdht` binary from a GitHub release

## Install from release

Pick the release and target asset from:

- <https://github.com/standard-librarian/mdht/releases>

The asset pattern is:

- `mdht_<version>_<os>_<arch>.tar.gz`

Example:

```bash
VERSION=v0.5.0-beta.1
ASSET="mdht_${VERSION#v}_linux_amd64.tar.gz"
curl -fsSL -o "${ASSET}" "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/${ASSET}"
tar -xzf "${ASSET}"
./mdht --help
```

## Create your first study loop

```bash
VAULT=/path/to/obsidian-vault

# Ingest one source.
mdht ingest file /path/to/paper.pdf --type paper --title "Raft Paper" --topics distributed-systems --vault "$VAULT"

# Discover source id.
mdht source list --vault "$VAULT"

# Start an explicit session.
mdht session start --source-id <source-id> --goal "read sections 1-3" --vault "$VAULT"

# Open reader (or TUI).
mdht reader open --source-id <source-id> --mode auto --page 1 --vault "$VAULT"
mdht tui --vault "$VAULT"

# End session and apply cumulative progress delta.
mdht session end --outcome "finished sections 1-3" --delta-progress 15 --vault "$VAULT"
```

Progress rule:

- `progress_after = clamp(progress_before + delta_progress, 0, 100)`

## Core command groups

- `ingest`: add file/url sources
- `source`: list/show source metadata
- `reader`: open content in markdown/pdf/external mode
- `session`: start/end study blocks
- `reindex`: rebuild projections from vault state
- `plugin`: plugin SDK commands
- `collab`: collaboration daemon and peer operations

## Troubleshooting basics

1. `mdht --help` and `mdht <group> --help` for flag-level help.
2. Confirm `--vault` points to the expected vault.
3. Run `mdht reindex --vault <path>` if projection data looks stale.
4. For collaboration issues, see [public beta collab guide](public-beta-collab.md) and [operator runbook](runbooks/collab-operator.md).

