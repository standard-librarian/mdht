---
name: mdht-claude
description: Claude-oriented wrapper for mdht engineering tasks and repo-local skill packaging. Use when working on mdht with Claude and when preparing the repository skill pack for Claude compatibility.
---

# mdht Claude Wrapper

## Use this wrapper to

- Apply `mdht-core` architecture and test workflow.
- Keep module boundaries explicit and validated.
- Maintain Claude skill-pack compatibility from `skills/` and `claude-plugin/`.

## Execution order

1. Load `../mdht-core/SKILL.md` and references.
2. Implement module-layer compliant changes.
3. Validate behavior and architecture with tests.
4. Validate skill files with `scripts/validate-skills.sh`.

## References

- `references/claude-packaging.md`
