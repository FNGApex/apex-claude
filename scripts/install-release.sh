#!/usr/bin/env bash
#
# install-release.sh — install Apex Claude on Linux/macOS from a prebuilt release.
#
# The Unix counterpart to scripts/install.ps1. Where scripts/install.sh builds
# the apex binary from source (and so needs go + make), this script downloads
# the prebuilt release bundle published by scripts/publish.ps1 — so a fresh
# Linux or macOS box needs no Go toolchain, just curl (or wget) and python3.
#
# Designed for one-line install:
#
#     curl -fsSL https://github.com/FNGApex/apex-claude/releases/latest/download/install-release.sh | bash
#
# Because a piped install cannot take flags, overrides are read from env vars:
#     APEX_VERSION       pin a release tag (default: latest), e.g. v0.2.0
#     CLAUDE_CONFIG_DIR  install root      (default: $HOME/.claude)
#
# What it does (all idempotent, safe to re-run):
#   1. Detects OS/arch and downloads apex-claude-<os>-<arch>.zip from the release.
#   2. Removes any prior PLUGIN install of Apex (migration).
#   3. Copies artifacts into ~/.claude/{commands,agents,skills,output-styles}.
#   4. Installs the binary into ~/.claude/bin/apex.
#   5. Wires the PreToolUse + SessionStart hooks into ~/.claude/settings.json,
#      preserving every other setting.
#
# It does NOT touch ~/.claude/CLAUDE.md — the Apex spine is opt-in. See README.
# Remove later with scripts/uninstall.sh.
#
# Usage:
#   curl -fsSL .../install-release.sh | bash
#   APEX_VERSION=v0.2.0 bash scripts/install-release.sh
#   scripts/install-release.sh --help
#
set -euo pipefail

REPO="FNGApex/apex-claude"
CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
VERSION="${APEX_VERSION:-latest}"
APEX_BIN="$CONFIG_DIR/bin/apex"

PLUGIN_ID="apex-claude@apex-claude"
MARKETPLACE="apex-claude"

for arg in "$@"; do
  case "$arg" in
    -h|--help)
      sed -n '2,/^set -euo/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//; /^set -euo/d'
      exit 0 ;;
    *)
      echo "install-release.sh: unknown argument '$arg' (try --help)" >&2
      exit 2 ;;
  esac
done

say()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# --- preflight ---------------------------------------------------------------
# python3 is required for the settings.json hook merge, and doubles as the zip
# extractor (`python3 -m zipfile`) when `unzip` is absent — so the only other
# hard requirement is a downloader.
have python3 || die "'python3' is required to merge hooks into settings.json"
have curl || have wget || die "need 'curl' or 'wget' to download the release bundle"

# --- detect platform ---------------------------------------------------------
os_raw="$(uname -s)"
case "$os_raw" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) die "unsupported OS '$os_raw' — this installer targets Linux and macOS (use install.ps1 on Windows)" ;;
esac

arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "unsupported architecture '$arch_raw' — release matrix covers amd64 and arm64 only" ;;
esac

ASSET="apex-claude-$OS-$ARCH.zip"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
else
  URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"
fi

# --- 1. download + extract ---------------------------------------------------
TMP="$(mktemp -d "${TMPDIR:-/tmp}/apex-install.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT
ZIP="$TMP/$ASSET"
SRC="$TMP/bundle"

say "Downloading $VERSION bundle ($OS/$ARCH)"
if have curl; then
  curl -fsSL "$URL" -o "$ZIP" \
    || die "download failed from $URL — check the version tag and that a release exists"
else
  wget -qO "$ZIP" "$URL" \
    || die "download failed from $URL — check the version tag and that a release exists"
fi

say "Extracting"
mkdir -p "$SRC"
if have unzip; then
  unzip -q "$ZIP" -d "$SRC"
else
  python3 -m zipfile -e "$ZIP" "$SRC"
fi
[ -x "$SRC/apex" ] || chmod +x "$SRC/apex" 2>/dev/null || true
[ -f "$SRC/apex" ] || die "bundle is missing the apex binary — corrupt or wrong asset"

# --- 2. drop any prior plugin install (migration) ----------------------------
if have claude && claude plugin list 2>/dev/null | grep -q "$PLUGIN_ID"; then
  say "Removing prior plugin install of Apex (switching to loose artifacts)"
  claude plugin uninstall "$PLUGIN_ID" || true
  claude plugin marketplace remove "$MARKETPLACE" 2>/dev/null || true
fi

# --- 3. copy artifacts -------------------------------------------------------
say "Installing artifacts into $CONFIG_DIR/"
mkdir -p "$CONFIG_DIR"/{commands,agents,skills,output-styles,bin}
cp "$SRC"/commands/ax-*.md       "$CONFIG_DIR/commands/"
cp "$SRC"/agents/ax-*.md         "$CONFIG_DIR/agents/"
cp "$SRC"/output-styles/apex.md  "$CONFIG_DIR/output-styles/apex.md"
# Repair residue from a prior buggy install: a trailing-slash `cp -r src/ dest/`
# collapsed every skill into a single top-level skills/SKILL.md. Skills are
# always dir/SKILL.md, so a file directly under skills/ is invalid layout —
# remove it so the orphan doesn't linger.
rm -f "$CONFIG_DIR/skills/SKILL.md"
for d in "$SRC"/skills/ax-*/; do
  dest="$CONFIG_DIR/skills/$(basename "$d")"
  rm -rf "$dest"
  # Copy to an explicit dest dir, NOT into skills/ with a trailing-slash source:
  # BSD/macOS `cp -r src/ dest/` copies src's *contents*, collapsing every skill
  # into one. Naming the dest dir copies the directory itself on both BSD + GNU.
  cp -r "$d" "$dest"
done

# --- 4. binary ---------------------------------------------------------------
say "Installing binary into $APEX_BIN"
cp "$SRC/apex" "$APEX_BIN"
chmod +x "$APEX_BIN"

# The binary lives under ~/.claude/bin, which is not on a default PATH — so
# `apex` won't resolve in a fresh shell unless the user wires it. We never
# mutate the user's rc; we only print the exact line to add. Empty ON_PATH =>
# print guidance at the end.
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

$(printf '\033[1;32m✔ Apex Claude installed (loose artifacts, prebuilt %s).\033[0m' "$VERSION")

  commands : $(ls "$CONFIG_DIR"/commands/ax-*.md | wc -l | tr -d ' ')  → $CONFIG_DIR/commands/
  agents   : $(ls "$CONFIG_DIR"/agents/ax-*.md | wc -l | tr -d ' ')  → $CONFIG_DIR/agents/
  skills   : $(ls -d "$CONFIG_DIR"/skills/ax-*/ | wc -l | tr -d ' ')   → $CONFIG_DIR/skills/
  style    : Apex → $CONFIG_DIR/output-styles/apex.md

Next steps:
  • Restart Claude Code so /ax-* commands, agents, skills, and hooks load.
  • Activate the output style: /output-style Apex
  • Remove later with: scripts/uninstall.sh
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
