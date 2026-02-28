# mdht

`mdht` (Markdown Terminal Hub Tool) is a local-first terminal app for study workflows on top of an Obsidian vault.

`mdht` provides:

- source ingest and vault-backed notes
- explicit study session lifecycle with cumulative progress tracking
- reader flows for markdown/pdf and external launch for video/course sources
- plugin SDK v1 (`command`, `analyze`, `fullscreen_tty`)
- peer-to-peer collaboration transport for mdht-managed content
- tab-based TUI with full-screen views, scrollable lists, markdown rendering, and a command palette

## Public Beta Status

- Platform support: macOS and Linux.
- Collaboration scope: mdht-managed fields/blocks only (`source/*`, `session/*`, `topic/*`).
- Vault markdown is source of truth; SQLite is a projection/index.

## Install

### Build from source (Go 1.25+)

```bash
git clone https://github.com/standard-librarian/mdht.git
cd mdht
go build -o mdht ./cmd/mdht
./mdht --help
```

Move the binary somewhere on your `$PATH`:

```bash
mv mdht /usr/local/bin/
```

### GitHub Release

Pre-built release archives are published per tag (`v*`) at
[GitHub Releases](https://github.com/standard-librarian/mdht/releases)
for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`.

Each release contains `mdht_<version>_<os>_<arch>.tar.gz` and `checksums.txt`.

Example (macOS arm64):

```bash
VERSION=v0.6.0-beta.1
ASSET="mdht_${VERSION#v}_darwin_arm64.tar.gz"
curl -fsSL -o "${ASSET}" "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/${ASSET}"
curl -fsSL -o checksums.txt "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/checksums.txt"
grep " ${ASSET}$" checksums.txt | shasum -a 256 -c -
tar -xzf "${ASSET}"
./mdht --help
```

Example (Linux amd64):

```bash
VERSION=v0.6.0-beta.1
ASSET="mdht_${VERSION#v}_linux_amd64.tar.gz"
curl -fsSL -o "${ASSET}" "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/${ASSET}"
curl -fsSL -o checksums.txt "https://github.com/standard-librarian/mdht/releases/download/${VERSION}/checksums.txt"
grep " ${ASSET}$" checksums.txt | sha256sum -c -
tar -xzf "${ASSET}"
./mdht --help
```

## Configuration

### `MDHT_VAULT` environment variable

Set `MDHT_VAULT` once in your shell profile to avoid passing `--vault` on every command:

```bash
export MDHT_VAULT=/path/to/obsidian-vault
```

The `--vault` flag still works and takes precedence over the environment variable.

## Quick Start

```bash
export MDHT_VAULT=/path/to/obsidian-vault

# 1) ingest one source
mdht ingest file /path/to/source.pdf --type paper --title "Distributed Systems" --topics systems,consensus

# 2) list sources and pick an id
mdht source list

# 3) start a session, study, and end with delta
mdht session start --source-id <source-id> --goal "chapter 1"
mdht tui
mdht session end --outcome "finished chapter 1" --delta-progress 12.5
```

## TUI Key Bindings

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Cycle tabs (Library → Reader → Graph → Collab → Plugins) |
| `enter` | Open selected source in Reader (Library tab) |
| `s` | Start session for selected source (Library tab) |
| `←` / `→` | Previous / next PDF page (Reader tab) |
| `↑` / `↓` | Scroll content |
| `/` | Filter list (Library, Graph, Collab tabs) |
| `:` | Open command palette |
| `?` | Toggle help overlay |
| `q` / `ctrl+c` | Quit |

### Command Palette (`:`)

| Command | Description |
|---------|-------------|
| `session:start` | Start session for selected source |
| `session:end <delta> <outcome>` | End the active session |
| `reader:open [mode] [page]` | Open selected source in reader |
| `reader:next-page` / `reader:prev-page` | Navigate PDF pages |
| `source:open-external` | Open selected source in default app |
| `plugin:exec <plugin> <command> [json]` | Execute a plugin command |
| `plugin:commands` | Switch to Plugins tab |
| `plugin:tty <plugin> <command> [json]` | Run a fullscreen TTY plugin |
| `collab:status` / `collab:peers` / `collab:activity` / `collab:conflicts` | Switch to Collab tab |
| `collab:resolve <id> <local\|remote\|merge>` | Resolve a sync conflict |
| `collab:sync-now` / `collab:sync-health` | Trigger sync / check health |

## Command Groups

- `tui`: primary terminal UI.
- `ingest`, `source`, `reader`, `session`, `reindex`: core study loop.
- `graph`: knowledge graph exploration (`topics`, `neighbors`, `search`, `path`).
- `plugin`: plugin discovery, validation, and execution.
- `collab`: daemon lifecycle, workspace/peer management, activity/presence/conflicts, sync, snapshot, metrics, and connectivity diagnostics (`net status|probe`, `peer dial|latency|timeline`, `presence`, `sync health`).

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
git tag v0.6.0-beta.1
git push origin v0.6.0-beta.1
```

The release workflow builds all four target archives, publishes `checksums.txt`, and creates a GitHub release entry.
