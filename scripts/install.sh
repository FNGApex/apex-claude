#!/usr/bin/env bash
#
# install.sh — deploy Apex Claude as loose user-level artifacts.
#
# Apex installs its commands/agents/skills/output-style directly into
# ~/.claude/ (NOT as a Claude Code plugin). User-level artifacts are not
# namespaced, so commands appear as bare /ax-* instead of /apex-claude:ax-*.
# The tradeoff: no plugin enable/disable/update lifecycle — this script owns
# install, and scripts/uninstall.sh owns removal.
#
# What it does (all idempotent, safe to re-run):
#   1. Builds the apex backbone binary.
#   2. Removes any prior PLUGIN install of Apex (migration — avoids /ax-* and
#      /apex-claude:ax-* showing up as duplicates).
#   3. Copies artifacts into ~/.claude/{commands,agents,skills,output-styles}.
#   4. Installs the binary into ~/.claude/bin/apex.
#   5. Wires the PreToolUse + SessionStart hooks into ~/.claude/settings.json,
#      preserving every other setting.
#
# It does NOT touch ~/.claude/CLAUDE.md — the Apex spine is opt-in. See README.
#
# Usage:
#   scripts/install.sh            # build + install
#   scripts/install.sh --release  # build the full cross-compile matrix first
#   scripts/install.sh --no-build # skip the build, install the existing binary
#   scripts/install.sh --help
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
APEX_BIN="$CONFIG_DIR/bin/apex"

PLUGIN_ID="apex-claude@apex-claude"
MARKETPLACE="apex-claude"

BUILD=1
RELEASE=0

for arg in "$@"; do
  case "$arg" in
    --release)  RELEASE=1 ;;
    --no-build) BUILD=0 ;;
    -h|--help)
      sed -n '2,/^set -euo/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//; /^set -euo/d'
      exit 0 ;;
    *)
      echo "install.sh: unknown argument '$arg' (try --help)" >&2
      exit 2 ;;
  esac
done

say()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# --- preflight ---------------------------------------------------------------
have python3 || die "'python3' is required to merge hooks into settings.json"
if [ "$BUILD" -eq 1 ]; then
  have go   || die "'go' is not on PATH — needed to build the apex binary (or pass --no-build)"
  have make || die "'make' is not on PATH — needed to build (or pass --no-build)"
fi

cd "$REPO_ROOT"

# --- 1. build ----------------------------------------------------------------
if [ "$BUILD" -eq 1 ]; then
  say "Cleaning bin/ then building"
  rm -rf bin
  make build
  [ "$RELEASE" -eq 1 ] && make release
  [ -x bin/apex ] || die "build did not produce an executable bin/apex"
else
  say "Skipping build (--no-build)"
  [ -x bin/apex ] || die "--no-build set but bin/apex is missing — build it first"
fi

# --- 2. drop any prior plugin install (migration) ----------------------------
if have claude && claude plugin list 2>/dev/null | grep -q "$PLUGIN_ID"; then
  say "Removing prior plugin install of Apex (switching to loose artifacts)"
  claude plugin uninstall "$PLUGIN_ID" || true
  claude plugin marketplace remove "$MARKETPLACE" 2>/dev/null || true
fi

# --- 3. copy artifacts -------------------------------------------------------
say "Installing artifacts into $CONFIG_DIR/"
mkdir -p "$CONFIG_DIR"/{commands,agents,skills,output-styles,bin}
cp commands/ax-*.md          "$CONFIG_DIR/commands/"
cp agents/ax-*.md            "$CONFIG_DIR/agents/"
cp output-styles/protocol.md "$CONFIG_DIR/output-styles/apex.md"
for d in skills/ax-*/; do
  rm -rf "$CONFIG_DIR/skills/$(basename "$d")"
  cp -r "$d" "$CONFIG_DIR/skills/"
done

# --- 4. binary ---------------------------------------------------------------
say "Installing binary into $APEX_BIN"
cp bin/apex "$APEX_BIN"
chmod +x "$APEX_BIN"

# Detect whether the install dir is reachable on PATH. The binary lives under
# ~/.claude/bin, which is not on a default PATH — so `apex` won't resolve in a
# fresh shell unless the user wires it. We never mutate the user's rc; we only
# tell them the exact line to add. Empty ON_PATH => print guidance at the end.
BIN_DIR="$(dirname "$APEX_BIN")"
ON_PATH=""
case ":$PATH:" in
  *":$BIN_DIR:"*) ON_PATH=1 ;;
esac

# --- 5. wire hooks (preserve all other settings) -----------------------------
say "Wiring hooks into $CONFIG_DIR/settings.json"
python3 - "$CONFIG_DIR/settings.json" "$APEX_BIN" <<'PY'
import json, sys
settings_path, apex_bin = sys.argv[1], sys.argv[2]
try:
    with open(settings_path) as f:
        data = json.load(f)
except FileNotFoundError:
    data = {}
except json.JSONDecodeError as e:
    sys.exit(f"settings.json is not valid JSON ({e}); fix it by hand and re-run")

hooks = data.setdefault("hooks", {})

def is_apex(group):
    return any("apex hooks" in h.get("command", "") for h in group.get("hooks", []))

# strip any prior apex groups so re-runs don't stack duplicates
for event in ("PreToolUse", "SessionStart"):
    if event in hooks:
        hooks[event] = [g for g in hooks[event] if not is_apex(g)]

hooks.setdefault("PreToolUse", []).append({
    "matcher": "Bash",
    "hooks": [{"type": "command", "command": f"{apex_bin} hooks pre-bash"}],
})
hooks.setdefault("SessionStart", []).append({
    "hooks": [{"type": "command", "command": f"{apex_bin} hooks session-start"}],
})

with open(settings_path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
print(f"  hooks wired -> {apex_bin}")
PY

# --- done --------------------------------------------------------------------
cat <<EOF

$(printf '\033[1;32m✔ Apex Claude installed (loose artifacts).\033[0m')

  commands : $(ls commands/ax-*.md | wc -l | tr -d ' ')  → $CONFIG_DIR/commands/
  agents   : $(ls agents/ax-*.md | wc -l | tr -d ' ')  → $CONFIG_DIR/agents/
  skills   : $(ls -d skills/ax-*/ | wc -l | tr -d ' ')   → $CONFIG_DIR/skills/
  style    : Apex → $CONFIG_DIR/output-styles/apex.md

Next steps:
  • Restart Claude Code so /ax-* commands, agents, skills, and hooks load.
  • Activate the output style: /output-style Apex
  • Remove later with: scripts/uninstall.sh

Re-run this script after any code change to refresh the installed binary.
EOF

if [ -z "$ON_PATH" ]; then
  cat <<EOF
$(printf '\033[1;33m! %s is not on your PATH\033[0m' "$BIN_DIR")
  The 'apex' command resolves for Claude Code's hooks (they call the full path),
  but to run 'apex' yourself, add this line to your shell rc (~/.bashrc or
  ~/.zshrc) and start a new shell:

    export PATH="$BIN_DIR:\$PATH"

EOF
fi
