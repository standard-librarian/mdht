---
name: mdht-codex
description: Codex-oriented wrapper for mdht engineering tasks. Use when operating on this repository with Codex to apply mdht-core rules, command conventions, and strict module boundary checks.
---

# mdht Codex Wrapper

## Use this wrapper to

- Apply `mdht-core` workflow before implementation.
- Keep edits scoped to one module at a time when possible.
- Route cross-module calls through `port/in` interfaces.
- Run architecture and behavior tests before completion.

## Execution order

1. Load `../mdht-core/SKILL.md` and relevant references.
2. Implement feature/fix with module-first layering.
3. Run `go test ./...` and architecture tests.
4. Report changed files and test outcomes.

## References

- `references/codex-checklist.md`
