# mdht Documentation

This folder is the public documentation set for `mdht`.

## Start here

1. [Getting started](getting-started.md)
2. [Collaboration public beta guide](public-beta-collab.md)
3. [Release and upgrade guide](releases.md)

## Reference docs

- [Plugin SDK v1](plugin-sdk-v1.md)
- [Collab operator runbook](runbooks/collab-operator.md)
- [Architecture decisions (ADR)](adr/)

## Product boundaries

- Supported platforms: macOS and Linux.
- Vault markdown is source of truth.
- Collaboration sync is limited to mdht-managed content:
  - `source/*` managed frontmatter + managed links block
  - `session/*` managed frontmatter + managed body sections
  - `topic/*` mdht-managed topic/link content

