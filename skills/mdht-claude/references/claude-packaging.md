# Claude packaging notes

- Keep skill directories in `skills/` with valid `SKILL.md` frontmatter.
- Keep `claude-plugin/manifest.json` aligned with local skill layout.
- Install symlinks with `scripts/install-skills.sh claude`.
- Validate all skills with `scripts/validate-skills.sh`.
