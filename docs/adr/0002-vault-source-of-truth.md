# ADR 0002: Obsidian Vault Is Source of Truth

## Status
Accepted

## Decision
- Markdown in the Obsidian vault is canonical state.
- SQLite is a denormalized projection for search/indexing.
- `mdht reindex` must reconstruct projections from vault markdown alone.
- mdht edits only managed blocks and frontmatter fields it owns.

## Consequences
- Better interoperability with Obsidian and git workflows.
- Reindex provides recovery path for index corruption.
