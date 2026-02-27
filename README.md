# mdht

`mdht` (Markdown Terminal Hub Tool) is a local-first terminal app for study workflows on top of an Obsidian vault.

`mdht` provides:

- source ingest and vault-backed notes
- explicit study session lifecycle with cumulative progress tracking
- reader flows for markdown/pdf and external launch for video/course sources
- plugin SDK v1 (`command`, `analyze`, `fullscreen_tty`)
- peer-to-peer collaboration transport for mdht-managed content

## Public Beta Status

- Platform support: macOS and Linux.
- Collaboration scope: mdht-managed fields/blocks only (`source/*`, `session/*`, `topic/*`).
- Vault markdown is source of truth; SQLite is a projection/index.

## Install (GitHub Release)

Release assets are published per tag (`v*`) in [GitHub Releases](https://github.com/standard-librarian/mdht/releases).

Available targets:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Each release contains:

- `mdht_<version>_<os>_<arch>.tar.gz`
- `checksums.txt`

Example (macOS arm64):

```bash
VERSION=v0.5.0-beta.1
ASSET="mdht_${VERSION#v}_darwin_arm64.tar.gz"
curl -fsSL -o "${ASSET}" "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/${ASSET}"
curl -fsSL -o checksums.txt "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/checksums.txt"
grep " ${ASSET}$" checksums.txt | shasum -a 256 -c -
tar -xzf "${ASSET}"
./mdht --help
```

Example (Linux amd64):

```bash
VERSION=v0.5.0-beta.1
ASSET="mdht_${VERSION#v}_linux_amd64.tar.gz"
curl -fsSL -o "${ASSET}" "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/${ASSET}"
curl -fsSL -o checksums.txt "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/checksums.txt"
grep " ${ASSET}$" checksums.txt | sha256sum -c -
tar -xzf "${ASSET}"
./mdht --help
```

## Quick Start

```bash
VAULT=/path/to/obsidian-vault

# 1) ingest one source
mdht ingest file /path/to/source.pdf --type paper --title "Distributed Systems" --topics systems,consensus --vault "$VAULT"

# 2) list sources and pick an id
mdht source list --vault "$VAULT"

# 3) start a session, study, and end with delta
mdht session start --source-id <source-id> --goal "chapter 1" --vault "$VAULT"
mdht tui --vault "$VAULT"
mdht session end --outcome "finished chapter 1" --delta-progress 12.5 --vault "$VAULT"
```

## Command Groups

- `tui`: primary terminal UI.
- `ingest`, `source`, `reader`, `session`, `reindex`: core study loop.
- `plugin`: plugin discovery, validation, and execution.
- `collab`: daemon lifecycle, workspace/peer approval, activity/conflicts, sync, snapshot, metrics, and connectivity diagnostics (`net status|probe`, `peer dial|latency`, `sync health`).

Use `mdht <group> --help` for full flags and examples.

## Documentation

- [Docs index](docs/index.md)
- [Getting started](docs/getting-started.md)
- [Collaboration public beta guide](docs/public-beta-collab.md)
- [Release and upgrade guide](docs/releases.md)
- [Plugin SDK v1](docs/plugin-sdk-v1.md)
- [Collab operator runbook](docs/runbooks/collab-operator.md)

## Architecture and Quality

Layer contract per module:

`adapter/in -> usecase -> service -> port/out -> adapter/out`

Quality gates:

- `golangci-lint`
- architecture boundary tests (`internal/architecture`)
- race-enabled tests
- domain+usecase aggregate coverage gate (`>= 85%`)
- plugin host integration tests (reference plugin fixture)
- deterministic dependency checks (`go mod tidy`, `go mod verify`, clean `go.mod/go.sum` diff)

## Maintainer Release Flow

```bash
git tag v0.5.0-beta.1
git push origin v0.5.0-beta.1
```

The release workflow builds all four target archives, publishes `checksums.txt`, and creates a GitHub release entry.
