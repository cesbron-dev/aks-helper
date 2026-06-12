#!/usr/bin/env bash
#
# Install aks-helper globally: the binary on your PATH and the agent skill into
# the skills directories that coding agents read. Safe to re-run (idempotent).
#
# Usage:
#   scripts/install.sh                       # binary + skill, global, all agents
#   scripts/install.sh --skill-only          # just the agent skill
#   scripts/install.sh --binary-only         # just the binary
#   scripts/install.sh --scope local         # into ./ (a project) instead of ~/
#   scripts/install.sh --agent copilot       # only one agent (claude|copilot|agents)
#
# Skill locations (per the Agent Skills spec):
#   global  claude=~/.claude/skills  copilot=~/.copilot/skills  agents=~/.agents/skills
#   local   claude=./.claude/skills  copilot=./.github/skills   agents=./.agents/skills
#
# Override the binary dir with $BINDIR (default: ${GOBIN:-$HOME/.local/bin}).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINDIR="${BINDIR:-${GOBIN:-$HOME/.local/bin}}"
SKILL_NAME="aks-access"
SKILL_SRC="$REPO_ROOT/.claude/skills/$SKILL_NAME"

scope="global"
agent="all"
do_binary=1
do_skill=1

usage() { sed -n '3,18p' "$0" | sed 's/^# \{0,1\}//'; }

while [ $# -gt 0 ]; do
  case "$1" in
    --skill-only)  do_binary=0 ;;
    --binary-only) do_skill=0 ;;
    --scope)       scope="${2:?}"; shift ;;
    --scope=*)     scope="${1#*=}" ;;
    --agent)       agent="${2:?}"; shift ;;
    --agent=*)     agent="${1#*=}" ;;
    -h|--help)     usage; exit 0 ;;
    *)             echo "unknown option: $1 (try --help)" >&2; exit 2 ;;
  esac
  shift
done

case "$scope" in global|local) ;; *) echo "invalid --scope: $scope" >&2; exit 2 ;; esac
case "$agent" in
  all)                  agents="claude copilot agents" ;;
  claude|copilot|agents) agents="$agent" ;;
  *) echo "invalid --agent: $agent (claude|copilot|agents|all)" >&2; exit 2 ;;
esac

# skill_base <agent> echoes the directory that contains per-skill folders.
skill_base() {
  if [ "$scope" = global ]; then
    case "$1" in
      claude)  echo "$HOME/.claude/skills" ;;
      copilot) echo "$HOME/.copilot/skills" ;;
      agents)  echo "$HOME/.agents/skills" ;;
    esac
  else
    case "$1" in
      claude)  echo "$PWD/.claude/skills" ;;
      copilot) echo "$PWD/.github/skills" ;;
      agents)  echo "$PWD/.agents/skills" ;;
    esac
  fi
}

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
  if [ ! -f "$SKILL_SRC/SKILL.md" ]; then
    echo "error: skill not found at $SKILL_SRC" >&2
    return 1
  fi
  for a in $agents; do
    local dest="$(skill_base "$a")/$SKILL_NAME"
    if [ "$(cd "$SKILL_SRC" && pwd)" = "$dest" ]; then
      echo "skipping $a: source and destination are the same ($dest)"
      continue
    fi
    mkdir -p "$dest"
    cp -R "$SKILL_SRC/." "$dest/"
    echo "Installed skill '$SKILL_NAME' ($a, $scope) -> $dest"
  done
}

[ "$do_binary" = 1 ] && install_binary
[ "$do_skill" = 1 ] && install_skill

echo
echo "Done."
if [ "$do_binary" = 1 ]; then
  echo "  - Enable shell integration (once):  eval \"\$(aks-helper shell-init bash)\""
fi
if [ "$do_skill" = 1 ]; then
  echo "  - The '$SKILL_NAME' skill is now available to your coding agent(s)."
fi
