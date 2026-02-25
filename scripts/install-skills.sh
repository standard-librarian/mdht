#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${1:-both}"

install_codex() {
  mkdir -p "$HOME/.codex/skills"
  for skill in mdht-core mdht-codex; do
    rm -rf "$HOME/.codex/skills/$skill"
    ln -s "$ROOT/skills/$skill" "$HOME/.codex/skills/$skill"
  done
}

install_claude() {
  mkdir -p "$HOME/.claude/skills"
  for skill in mdht-core mdht-claude; do
    rm -rf "$HOME/.claude/skills/$skill"
    ln -s "$ROOT/skills/$skill" "$HOME/.claude/skills/$skill"
  done
}

case "$TARGET" in
  codex) install_codex ;;
  claude) install_claude ;;
  both)
    install_codex
    install_claude
    ;;
  *)
    echo "usage: $0 [codex|claude|both]" >&2
    exit 1
    ;;
esac

echo "skills installed for target: $TARGET"
