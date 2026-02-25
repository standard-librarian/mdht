#!/usr/bin/env bash
set -euo pipefail

go test ./...
go test ./... -race
go test ./internal/architecture -run TestHexagonalLayerImports
