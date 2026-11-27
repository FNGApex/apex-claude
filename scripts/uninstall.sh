#!/bin/sh
# uninstall.sh -- remove what install.sh put on this machine. POSIX sh; Windows counterpart: uninstall.ps1.
# Only what apex owns goes: skill folders with the .apex-installed mark, an output style whose
# frontmatter is `name: Apex`, mods installed as <mod>@apex, and the binary and manifest.
# Safe to run twice. Options: --prefix DIR, --config-dir DIR, --skills-dir DIR (an extra directory to sweep).

here=$(cd "$(dirname "$0")" && pwd) || exit 1
APEX_CONF=$here/workflows.conf
. "$here/apex-install-common.sh"

PREFIX= CONFIG_DIR= SKILLS_DIR=
while [ $# -gt 0 ]; do
  case $1 in
    --prefix | --config-dir | --skills-dir)
      [ $# -ge 2 ] || { echo "error: $1 needs a value" >&2; exit 64; }
      case $1 in --prefix) PREFIX=$2 ;; --config-dir) CONFIG_DIR=$2 ;; --skills-dir) SKILLS_DIR=$2 ;; esac
      shift
      ;;
    --yes | --non-interactive) ;;
    -h | --help) sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "error: unknown option $1" >&2; exit 64 ;;
  esac
  shift
done

[ -n "$HOME" ] || die 64 "HOME is not set"
MANIFEST=${XDG_CONFIG_HOME:-$HOME/.config}/apex/manifest
# Config dir: flag, else the one the install recorded, else the environment.
[ -n "$CONFIG_DIR" ] || CONFIG_DIR=$(mf_get config)
apex_resolve_env
PREFIX=${PREFIX:-$HOME/.local/bin}
OLD_BIN=$(mf_get bin)

# Every directory a skill could be in: the manifest's, the preset harnesses', and --skills-dir.
while IFS= read -r d; do [ -z "$d" ] || remove_marked "$d"; done <<EOF
$(mf_rows | cut -f2)
$HOME/.agents/skills
$CONFIG_DIR/skills
${CODEX_HOME:-$HOME/.codex}/skills
${GEMINI_CONFIG_DIR:-$HOME/.gemini/config}/skills
$SKILLS_DIR
EOF

[ -z "$OLD_BIN" ] || rm -f "$OLD_BIN"
rm -f "$PREFIX/apex" "$PREFIX/apex.exe"
remove_style
remove_mods
rm -f "$MANIFEST"
rmdir "$(dirname "$MANIFEST")" 2>/dev/null
echo "[ok] apex removed. Restart your harness."
exit 0
