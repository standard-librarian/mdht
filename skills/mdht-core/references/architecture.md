# mdht architecture reference

## Layer order

`adapter/in -> usecase -> service -> port/out -> adapter/out`

## Module list

- `library`: ingest/update/reindex source notes and projections
- `reader`: markdown/pdf read flows and progress updates
- `session`: session lifecycle and note output
- `graph`: topic notes and link projections
- `plugin`: manifests, checksum, lifecycle health
- `collab`: local deterministic state export and merge scaffolding

## Non-negotiables

- `usecase` orchestrates services.
- `service` owns business rules.
- `domain` must not depend on adapters.
- cross-module dependencies go through `port/in` only.
