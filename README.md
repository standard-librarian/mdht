# mdht

`mdht` (Markdown Terminal Hub Tool) is a hexagonal, module-first TUI for studying books, papers, articles, videos, and courses from an Obsidian vault.

## Commands

```bash
mdht tui --vault /path/to/vault

mdht ingest file /path/to/source.pdf --type paper --title "Distributed Systems" --topics systems,consensus
mdht ingest url https://example.com/article --type article --topics ai

mdht source list --vault /path/to/vault
mdht source show --id <source-id> --vault /path/to/vault

mdht session start --source-id <source-id> --goal "chapter 1" --vault /path/to/vault
mdht session end --outcome "finished chapter" --delta-progress 12.5 --vault /path/to/vault

mdht reader open --source-id <source-id> --mode auto --page 1 --vault /path/to/vault
mdht reader open --source-id <source-id> --external --vault /path/to/vault

mdht reindex --vault /path/to/vault
mdht plugin list --vault /path/to/vault
mdht plugin doctor --vault /path/to/vault
mdht plugin commands --plugin <plugin-name> --vault /path/to/vault
mdht plugin exec --plugin <plugin-name> --command <command-id> --input-json '{}' --vault /path/to/vault
mdht plugin analyze --plugin <plugin-name> --command <command-id> --source-id <source-id> --input-json '{}' --vault /path/to/vault
mdht plugin tty --plugin <plugin-name> --command <command-id> --input-json '{}' --vault /path/to/vault

mdht collab daemon run --vault /path/to/vault
mdht collab daemon start --vault /path/to/vault
mdht collab daemon stop --vault /path/to/vault
mdht collab daemon status --vault /path/to/vault
mdht collab workspace init --name "team-a" --vault /path/to/vault
mdht collab workspace show --vault /path/to/vault
mdht collab peer add --addr /ip4/203.0.113.10/tcp/4001/p2p/<peer-id> --vault /path/to/vault
mdht collab peer remove --peer-id <peer-id> --vault /path/to/vault
mdht collab peer list --vault /path/to/vault
mdht collab status --vault /path/to/vault
mdht collab reconcile --vault /path/to/vault
mdht collab export-state --vault /path/to/vault
```

## Architecture

Each module follows strict layers:

`adapter/in -> usecase -> service -> port/out -> adapter/out`

Modules:

- `library`
- `reader`
- `session`
- `graph`
- `plugin`
- `collab`

## Quality

- `golangci-lint`
- architectural import tests (`internal/architecture`)
- race-enabled tests
- blocking domain+usecase aggregate coverage gate (`>= 85%`)
- plugin host integration suite against `plugins/reference`

## Plugin SDK v1

- Manifest schema: `schemas/plugin-manifest.schema.json`
- RPC contract: `schemas/plugin-rpc-v1.proto`
- Reference plugin: `plugins/reference`
- Example manifest: `plugins/reference/plugins.example.json`

## Deferred (Next Phase)

- plugin marketplace/distribution and remote sandboxing
