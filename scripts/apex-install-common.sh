# Sourced by install.sh and uninstall.sh (POSIX sh). apex-install-common.ps1 is the PowerShell twin:
# keep flags, marks, the manifest and exit codes in step. Callers set APEX_CONF (workflows.conf).
# Exit codes: 0 ok, 1 ownership or prerequisite conflict, 2 missing go or claude, 64 usage error.

APEX_MARK=.apex-installed
# Plugins read live from mods/ (the apex marketplace); the @apex id is the ownership mark.
APEX_MODS="token-spend usage-footer"
TAB=$(printf '\t')
case $(uname -s 2>/dev/null) in MINGW* | MSYS* | CYGWIN*) EXE=.exe ;; *) EXE= ;; esac

die() { code=$1; shift; echo "error: $*" >&2; exit "$code"; }
warn() { echo "warning: $*" >&2; }

# Settings every script resolves the same way: flag value, else environment, else default.
apex_resolve_env() {
  [ -n "$HOME" ] || die 64 "HOME is not set"
  CONFIG_DIR=${CONFIG_DIR:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}}
  export CLAUDE_CONFIG_DIR="$CONFIG_DIR" # claude plugin reads this, not --config-dir
  MANIFEST=${XDG_CONFIG_HOME:-$HOME/.config}/apex/manifest
}

# ---- workflows and harnesses ----

workflow_name() {
  case $1 in
    orchestration) echo "Orchestration-based Workflow" ;;
    multi-agent) echo "Multi-Agent Collaboration" ;;
    both) echo "Both workflows" ;;
  esac
}
valid_workflow() { case $1 in orchestration | multi-agent | both) return 0 ;; esac; return 1; }
valid_harness() { case $1 in agents | claude | codex | custom) return 0 ;; esac; return 1; }

conf_skills() { sed -n "s/^$1:[[:space:]]*//p" "$APEX_CONF" | tr -d '\r'; }
# One skill per line; "both" is the union with duplicates dropped, order kept.
workflow_skills() {
  case $1 in
    both) { conf_skills orchestration; conf_skills multi-agent; } | tr ' ' '\n' | awk 'NF && !seen[$0]++' ;;
    *) conf_skills "$1" | tr ' ' '\n' | awk 'NF' ;;
  esac
}

harness_dir() {
  case $1 in
    agents) printf '%s/.agents/skills' "$HOME" ;;
    claude) printf '%s/skills' "$CONFIG_DIR" ;;
    codex) printf '%s/skills' "${CODEX_HOME:-$HOME/.codex}" ;;
    custom) printf '%s' "$SKILLS_DIR" ;;
  esac
}

# Validates $HARNESS (comma list) and needs $SKILLS_DIR when it names custom.
check_harness_list() {
  [ -n "$HARNESS" ] || die 64 "--harness needs at least one of agents, claude, codex, custom"
  case ",$HARNESS," in *,,*) die 64 "--harness '$HARNESS' has an empty entry" ;; esac
  for h in $(echo "$HARNESS" | tr ',' ' '); do
    valid_harness "$h" || die 64 "unknown harness '$h' (agents, claude, codex, custom)"
    if [ "$h" = custom ] && [ -z "$SKILLS_DIR" ]; then die 64 "--harness custom needs --skills-dir DIR"; fi
  done
}

# "harness<TAB>dir" per line for $HARNESS, one line per distinct directory, order kept.
targets() {
  for h in $(echo "$HARNESS" | tr ',' ' '); do
    printf '%s\t%s\n' "$h" "$(harness_dir "$h")"
  done | awk -F'\t' '!seen[$2]++'
}

# ---- manifest and previous install ----

mf_get() { [ -f "$MANIFEST" ] && awk -F'\t' -v k="$1" '$1 == k { print $2; exit }' "$MANIFEST"; }
mf_rows() { [ -f "$MANIFEST" ] && awk -F'\t' '$1 == "skills" { print $2 "\t" $3 }' "$MANIFEST"; }

# An install made before the manifest existed left only marked skills under the config dir.
legacy_present() {
  [ -f "$MANIFEST" ] && return 1
  for d in "$CONFIG_DIR"/skills/*/; do [ -f "${d}$APEX_MARK" ] && return 0; done
  return 1
}
prev_rows() { if [ -f "$MANIFEST" ]; then mf_rows; elif legacy_present; then printf 'claude\t%s/skills\n' "$CONFIG_DIR"; fi; }
prev_workflow() { if [ -f "$MANIFEST" ]; then mf_get workflow; elif legacy_present; then echo multi-agent; fi; }
prev_harness() { prev_rows | awk -F'\t' '!seen[$1]++ { printf "%s%s", (n++ ? "," : ""), $1 }'; }

# ---- ownership marks ----

# Every marked skill, listed or not, so a skill dropped from skills/ leaves no stale copy behind.
remove_marked() {
  [ -d "$1" ] || return 0
  for d in "$1"/*/; do
    if [ -f "${d}$APEX_MARK" ]; then rm -rf "$d" || die 1 "cannot remove $d"; fi
  done
}

# The output style is one file, so its own frontmatter name is the ownership mark.
style_file() { printf '%s/output-styles/apex.md' "$CONFIG_DIR"; }
style_marked() { grep -q '^name: Apex[[:space:]]*$' "$1" 2>/dev/null; }

plugins_json() { printf '%s/plugins/installed_plugins.json' "$CONFIG_DIR"; }
plugin_installed() { grep -q "\"$1\"" "$(plugins_json)" 2>/dev/null; }
# Another marketplace's copy of the mod $1.
foreign_mod() { grep -o "\"$1@[^\"]*\"" "$(plugins_json)" 2>/dev/null | grep -qv '@apex"'; }

# Uninstalls the @apex mods present in installed_plugins.json, then the apex marketplace.
remove_mods() {
  found=
  for m in $APEX_MODS; do plugin_installed "$m@apex" && found="$found $m"; done
  [ -n "$found" ] || return 0
  if ! command -v claude >/dev/null 2>&1; then
    warn "claude is not on PATH; mods still installed:$found (claude plugin uninstall <mod>@apex)"
    return 0
  fi
  for m in $found; do claude plugin uninstall "$m@apex" || warn "claude plugin uninstall $m@apex failed"; done
  claude plugin marketplace remove apex || warn "claude plugin marketplace remove apex failed"
}

remove_style() {
  f=$(style_file)
  if [ -f "$f" ] && style_marked "$f"; then rm -f "$f" || die 1 "cannot remove $f"; fi
}
