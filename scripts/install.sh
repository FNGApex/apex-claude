#!/usr/bin/env bash
#
# install.sh — deploy the Apex Claude plugin onto a Unix-style system.
#
# What it does (all idempotent, safe to re-run):
#   1. Builds the apex backbone binary into bin/ (the plugin's hooks call it).
#   2. Registers this repo as a Claude Code marketplace (user scope).
#   3. (Re)installs the plugin so the freshly built binary lands in the cache.
#   4. Enables the plugin.
#   5. Installs the Apex output style into the output-style picker.
#
# Why force a reinstall (step 3): `claude plugin install` copies the working
# tree into ~/.claude/plugins/cache at install time, and `plugin update` is
# version-gated — it will NOT recopy a same-version build. Uninstall→install is
# the only path that guarantees a rebuilt binary reaches the cache.
#
# Usage:
#   scripts/install.sh            # build (single binary) + install
#   scripts/install.sh --release  # build the full cross-compile matrix first
#   scripts/install.sh --no-build # skip the build, deploy bin/ as-is
#   scripts/install.sh --help
#
set -euo pipefail

MARKETPLACE="apex-claude"
PLUGIN="apex-claude"
PLUGIN_ID="${PLUGIN}@${MARKETPLACE}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"

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
have claude || die "the 'claude' CLI is not on PATH — install Claude Code first"
if [ "$BUILD" -eq 1 ]; then
  have go   || die "'go' is not on PATH — needed to build the apex binary (or pass --no-build)"
  have make || die "'make' is not on PATH — needed to build (or pass --no-build)"
fi

cd "$REPO_ROOT"

# --- 1. build ----------------------------------------------------------------
if [ "$BUILD" -eq 1 ]; then
  say "Cleaning bin/ (drop stale artifacts so only fresh binaries ship)"
  rm -rf bin
  if [ "$RELEASE" -eq 1 ]; then
    say "Building release matrix (make build + make release)"
    make build
    make release
  else
    say "Building bin/apex (make build)"
    make build
  fi
  [ -x bin/apex ] || die "build did not produce an executable bin/apex"
else
  say "Skipping build (--no-build)"
  [ -x bin/apex ] || die "--no-build set but bin/apex is missing — build it first"
fi

# --- 2. marketplace ----------------------------------------------------------
say "Registering marketplace from $REPO_ROOT"
claude plugin marketplace add "$REPO_ROOT"

# --- 3. (re)install ----------------------------------------------------------
if claude plugin list 2>/dev/null | grep -q "$PLUGIN_ID"; then
  say "Plugin already installed — uninstalling to force a fresh copy"
  claude plugin uninstall "$PLUGIN_ID" || die "uninstall failed"
fi
say "Installing $PLUGIN_ID (user scope)"
claude plugin install "$PLUGIN_ID" --scope user

# --- 4. enable ---------------------------------------------------------------
# `claude plugin enable` exits 0 even when already enabled (it prints a ✘ but
# does not fail). Rather than scrape its wording, attempt the enable and then
# verify the end state from `plugin list` — exit-code driven, not string-driven.
say "Enabling $PLUGIN_ID"
claude plugin enable "$PLUGIN_ID" || true
if claude plugin list 2>/dev/null | grep -A3 -F "$PLUGIN_ID" | grep -qi "enabled"; then
  say "Confirmed $PLUGIN_ID is enabled"
else
  die "plugin did not reach an enabled state — check: claude plugin list"
fi

# --- 5. output style ---------------------------------------------------------
say "Installing Apex output style into $CONFIG_DIR/output-styles/"
mkdir -p "$CONFIG_DIR/output-styles"
cp output-styles/protocol.md "$CONFIG_DIR/output-styles/apex.md"

# --- done --------------------------------------------------------------------
cat <<EOF

$(printf '\033[1;32m✔ Apex Claude installed.\033[0m')

Next steps:
  • Restart Claude Code (or run /reload-plugins) so /ax-* commands load.
  • Activate the output style: /output-style Apex
  • Verify: claude plugin list   →   $PLUGIN_ID (enabled)

Re-run this script after any code change to refresh the cached binary.
EOF
