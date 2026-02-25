#!/usr/bin/env bash
set -euo pipefail

VALIDATOR="/Users/admin/.codex/skills/.system/skill-creator/scripts/quick_validate.py"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

python3 "$VALIDATOR" "$ROOT/skills/mdht-core"
python3 "$VALIDATOR" "$ROOT/skills/mdht-codex"
python3 "$VALIDATOR" "$ROOT/skills/mdht-claude"

echo "all mdht skills validated"
