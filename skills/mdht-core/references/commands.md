# mdht command reference

## Build and run

- `go run ./cmd/mdht tui --vault .`
- `go run ./cmd/mdht ingest file ./README.md --type article --title "Readme"`
- `go run ./cmd/mdht reindex --vault .`
- `go run ./cmd/mdht plugin doctor --vault .`
- `go run ./cmd/mdht collab export-state --vault .`

## Testing and validation

- `go test ./...`
- `go test ./... -race`
- `go test ./internal/architecture -run TestHexagonalLayerImports`
- `scripts/validate-skills.sh`
