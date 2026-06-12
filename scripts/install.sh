#!/usr/bin/env bash
#
# Install aks-helper globally: the binary on your PATH and the agent skill into
# Claude Code's personal skills directory. Safe to re-run (idempotent).
#
# Usage:
#   scripts/install.sh                 # binary + skill (default)
#   scripts/install.sh --skill-only    # install just the agent skill
#   scripts/install.sh --binary-only   # build + install just the binary
#
# Overridable via environment:
#   BINDIR       where to put the binary   (default: ${GOBIN:-$HOME/.local/bin})
#   SKILLS_DIR   Claude Code skills dir    (default: $HOME/.claude/skills)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINDIR="${BINDIR:-${GOBIN:-$HOME/.local/bin}}"
SKILLS_DIR="${SKILLS_DIR:-$HOME/.claude/skills}"
SKILL_NAME="aks-access"

do_binary=1
do_skill=1
case "${1:-}" in
  --skill-only)  do_binary=0 ;;
  --binary-only) do_skill=0 ;;
  -h|--help)     sed -n '3,13p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
  "")            ;;
  *)             echo "unknown option: $1 (try --help)" >&2; exit 2 ;;
esac

install_binary() {
  if ! command -v go >/dev/null 2>&1; then
    echo "error: 'go' is required to build aks-helper (or download a release binary)" >&2
    return 1
  fi
  mkdir -p "$BINDIR"
  echo "Building aks-helper -> $BINDIR/aks-helper"
  ( cd "$REPO_ROOT" && go build -trimpath -ldflags "-s -w" -o "$BINDIR/aks-helper" . )
  case ":$PATH:" in
    *":$BINDIR:"*) ;;
    *) echo "note: $BINDIR is not on your PATH — add it so 'aks-helper' resolves." >&2 ;;
  esac
}

install_skill() {
  local src="$REPO_ROOT/.claude/skills/$SKILL_NAME"
  local dest="$SKILLS_DIR/$SKILL_NAME"
  if [ ! -f "$src/SKILL.md" ]; then
    echo "error: skill not found at $src" >&2
    return 1
  fi
  mkdir -p "$dest"
  cp -R "$src/." "$dest/"
  echo "Installed skill '$SKILL_NAME' -> $dest"
}

[ "$do_binary" = 1 ] && install_binary
[ "$do_skill" = 1 ] && install_skill

echo
echo "Done."
if [ "$do_binary" = 1 ]; then
  echo "  - Enable shell integration (once):  eval \"\$(aks-helper shell-init bash)\""
fi
if [ "$do_skill" = 1 ]; then
  echo "  - The '$SKILL_NAME' skill is now available to Claude Code in every session."
fi
