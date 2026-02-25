# mdht Plugin SDK v1

## Contracts

- Manifest file: `<vault>/plugins/plugins.json`
- Manifest schema: `schemas/plugin-manifest.schema.json`
- RPC schema: `schemas/plugin-rpc-v1.proto`

## Required manifest fields

- `name`
- `version`
- `binary`
- `sha256` (lowercase hex, 64 chars)
- `enabled`
- `capabilities` (`command`, `analyze`, `fullscreen_tty`)

## RPC methods

- `GetMetadata`
- `ListCommands`
- `Execute`
- `PrepareTTY`

## Command kinds

- `command`
- `analyze`
- `fullscreen_tty`

## Timeouts and errors

- Host applies startup and call timeouts.
- Execution is denied when plugin is disabled, checksum mismatches, capability is missing, or command is unknown.

## Reference implementation

See `plugins/reference/main.go` for an end-to-end server implementation compatible with the host runtime.
