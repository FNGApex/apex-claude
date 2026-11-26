#!/bin/sh
# install.sh -- install apex and its workflow skills for one or more harnesses (see usage below).
# POSIX sh. Windows counterpart: install.ps1. Undo with uninstall.sh.

here=$(cd "$(dirname "$0")" && pwd) || exit 1
REPO=$(dirname "$here")
APEX_CONF=$here/workflows.conf
. "$here/apex-install-common.sh"

usage() {
  cat <<'EOF'
Usage: scripts/install.sh [options]

  --workflow W            orchestration (default), multi-agent, or both
  --harness H             agents (default), claude, codex, custom; comma list allowed
  --skills-dir DIR        skills directory for the custom harness (implies custom)
  --prefix DIR            where apex goes (default ~/.local/bin; must be on PATH)
  --config-dir DIR        Claude config root (default $CLAUDE_CONFIG_DIR or ~/.claude)
  --with-output-style[=no|yes]   install (or remove) the Apex output style
  --with-claude-mods[=no|yes]    install (or remove) the token-spend and usage-footer mods
  --check-ponytail        require the ponytail plugin
  --custom, --interactive ask for each choice
  --yes, --non-interactive  never ask
  --check                 run the preflight checks and exit, writing nothing
  --bin FILE              install this apex binary instead of building one with go
  -h, --help              this text

Without --workflow and --harness an interactive terminal is asked; otherwise the previous
install's choices are kept, else orchestration on agents. Exit: 0 ok, 1 conflict, 2 missing
go or claude, 64 usage.
EOF
}
usage_err() { echo "error: $*" >&2; echo "run scripts/install.sh --help" >&2; exit 64; }

WORKFLOW= HARNESS= SKILLS_DIR= PREFIX= CONFIG_DIR= STYLE= MODS= BIN=
PONYTAIL=0 CHECK=0 MODE= # MODE: ask, yes, or empty (ask only on a terminal)
while [ $# -gt 0 ]; do
  case $1 in
    --workflow | --harness | --skills-dir | --prefix | --config-dir | --bin)
      [ $# -ge 2 ] || usage_err "$1 needs a value"
      case $1 in
        --workflow) WORKFLOW=$2 ;;
        --harness) HARNESS=$2 ;;
        --skills-dir) SKILLS_DIR=$2 ;;
        --prefix) PREFIX=$2 ;;
        --config-dir) CONFIG_DIR=$2 ;;
        --bin) BIN=$2 ;;
      esac
      shift
      ;;
    --with-output-style | --with-output-style=yes) STYLE=yes ;;
    --with-output-style=no) STYLE=no ;;
    --with-claude-mods | --with-claude-mods=yes) MODS=yes ;;
    --with-claude-mods=no) MODS=no ;;
    --with-output-style=* | --with-claude-mods=*) usage_err "$1: use =yes or =no" ;;
    --check-ponytail) PONYTAIL=1 ;;
    --custom | --interactive) [ "$MODE" = yes ] || MODE=ask ;; # --yes wins, as in install.ps1
    --yes | --non-interactive) MODE=yes ;;
    --check) CHECK=1 ;;
    -h | --help) usage; exit 0 ;;
    *) usage_err "unknown option $1" ;;
  esac
  shift
done

apex_resolve_env
PREFIX=${PREFIX:-$HOME/.local/bin}
[ -z "$WORKFLOW" ] || valid_workflow "$WORKFLOW" || usage_err "unknown workflow '$WORKFLOW' (orchestration, multi-agent, both)"
[ -n "$(conf_skills orchestration)" ] && [ -n "$(conf_skills multi-agent)" ] || die 1 "$APEX_CONF does not list both workflows"
# A skills directory without an explicit harness means custom; beside one, it adds custom.
if [ -n "$SKILLS_DIR" ]; then
  case ",$HARNESS," in *,custom,*) ;; *) HARNESS=${HARNESS:+$HARNESS,}custom ;; esac
fi
[ -z "$HARNESS" ] || check_harness_list

# ---- decide what to install: flag, then prompt, then previous install, then default ----

ask() { printf '%s' "$1"; IFS= read -r REPLY || REPLY=; }
yes_no() { ask "$1 [y/N] "; case $REPLY in y | Y | yes | YES | Yes) return 0 ;; esac; return 1; }

pick_workflow() {
  def=1; case $PREV_WF in multi-agent) def=2 ;; both) def=3 ;; esac
  echo "Workflow:"
  echo "  1) $(workflow_name orchestration)"
  echo "  2) $(workflow_name multi-agent)"
  echo "  3) Both"
  ask "Choose [$def]: "
  case ${REPLY:-$def} in
    1 | orchestration) WORKFLOW=orchestration ;;
    2 | multi-agent) WORKFLOW=multi-agent ;;
    3 | both) WORKFLOW=both ;;
    *) die 64 "invalid workflow choice '$REPLY'" ;;
  esac
}

pick_harness() {
  def=${PREV_H:-agents}
  echo "Harness (comma separated for several):"
  echo "  1) agents   ~/.agents/skills"
  echo "  2) claude   $CONFIG_DIR/skills"
  echo "  3) codex    ${CODEX_HOME:-$HOME/.codex}/skills"
  echo "  4) custom   a directory you name"
  ask "Choose [$def]: "
  HARNESS=
  for tok in $(echo "${REPLY:-$def}" | tr ',' ' '); do
    case $tok in 1) tok=agents ;; 2) tok=claude ;; 3) tok=codex ;; 4) tok=custom ;; esac
    valid_harness "$tok" || die 64 "invalid harness choice '$tok'"
    HARNESS=${HARNESS:+$HARNESS,}$tok
  done
  case ",$HARNESS," in
    *,custom,*) if [ -z "$SKILLS_DIR" ]; then ask "Skills directory: "; SKILLS_DIR=$REPLY; fi ;;
  esac
  check_harness_list
}

PREV_WF=$(prev_workflow)
PREV_H=$(prev_harness)
PROMPT=0
if [ "$MODE" = ask ]; then PROMPT=1
elif [ "$MODE" != yes ] && { [ -z "$WORKFLOW" ] || [ -z "$HARNESS" ]; } && [ -t 0 ]; then PROMPT=1; fi

KEPT=0
if [ -z "$WORKFLOW" ]; then
  if [ "$PROMPT" = 1 ]; then pick_workflow
  elif [ -n "$PREV_WF" ]; then WORKFLOW=$PREV_WF; KEPT=1
  else WORKFLOW=orchestration; fi
fi
if [ -z "$HARNESS" ]; then
  if [ "$PROMPT" = 1 ]; then pick_harness
  elif [ -n "$PREV_H" ]; then
    HARNESS=$PREV_H; KEPT=1
    # A custom directory is not derivable from the name; take it from the previous install.
    if [ -z "$SKILLS_DIR" ]; then SKILLS_DIR=$(mf_rows | awk -F'\t' '$1 == "custom" { print $2; exit }'); fi
    check_harness_list
  else HARNESS=agents; fi
fi
[ "$KEPT" = 0 ] || echo "keeping workflow $WORKFLOW, harness $HARNESS from previous install; pass --workflow/--harness to change"

if [ "$PROMPT" = 1 ]; then
  case ",$HARNESS," in
    *,claude,*)
      [ -n "$STYLE" ] || { yes_no "Install the Apex output style?" && STYLE=yes; }
      [ -n "$MODS" ] || { yes_no "Install the Claude mods (token-spend, usage-footer)?" && MODS=yes; }
      [ "$PONYTAIL" = 1 ] || { yes_no "Require the ponytail plugin?" && PONYTAIL=1; }
      ;;
  esac
fi

TARGETS=$(targets)
SKILLS=$(workflow_skills "$WORKFLOW")
STYLE_FILE=$(style_file)
BIN_DEST=$PREFIX/apex$EXE

if [ "$PROMPT" = 1 ]; then
  echo "Will install:"
  echo "  workflow: $(workflow_name "$WORKFLOW") ($WORKFLOW)"
  echo "  skills:   $(echo $SKILLS)"
  while IFS=$TAB read -r h d; do echo "  harness:  $h -> $d"; done <<EOF
$TARGETS
EOF
  echo "  apex:     $BIN_DEST"
  [ -z "$STYLE" ] || echo "  output style: $STYLE"
  [ -z "$MODS" ] || echo "  claude mods:  $MODS"
  ask "Proceed? [Y/n] "
  case $REPLY in n | N | no | NO | No) echo "aborted; nothing written"; exit 1 ;; esac
fi

# ---- preflight: every check, zero writes ----

if [ -z "$BIN" ] && ! command -v go >/dev/null 2>&1; then die 2 "go is not on PATH, so apex cannot be built; install Go or pass --bin FILE"; fi
if [ "$MODS" = yes ] && ! command -v claude >/dev/null 2>&1; then die 2 "claude is not on PATH; --with-claude-mods needs the claude CLI"; fi
[ -z "$BIN" ] || [ -f "$BIN" ] || usage_err "--bin $BIN is not a file"
if [ "$PONYTAIL" = 1 ] && ! plugin_installed ponytail@ponytail; then
  die 1 "ponytail plugin is not installed: /plugin marketplace add DietrichGebert/ponytail, then /plugin install ponytail@ponytail"
fi
for l in "$CONFIG_DIR/bin/apex" "$CONFIG_DIR/bin/apex.exe"; do
  [ -e "$l" ] && die 1 "$l is a legacy Apex v0.3 install and shadows the new apex on PATH; remove it first: example/scripts/uninstall.sh"
done
for s in $SKILLS; do
  [ -f "$REPO/skills/$s/SKILL.md" ] || die 1 "skills/$s/SKILL.md is missing from $REPO"
done
while IFS=$TAB read -r h d; do
  for s in $SKILLS; do
    if [ -e "$d/$s" ] && [ ! -f "$d/$s/$APEX_MARK" ]; then die 1 "$d/$s exists and was not installed by apex; move it first"; fi
  done
done <<EOF
$TARGETS
EOF
if [ "$STYLE" = yes ]; then
  [ -f "$REPO/output-styles/apex.md" ] || die 1 "output-styles/apex.md is missing from $REPO"
  if [ -e "$STYLE_FILE" ] && ! style_marked "$STYLE_FILE"; then die 1 "$STYLE_FILE exists and was not installed by apex; move it first"; fi
fi
if [ "$MODS" = yes ]; then
  for m in $APEX_MODS; do
    if foreign_mod "$m"; then die 1 "another $m plugin is installed; uninstall it first: claude plugin uninstall <$m@marketplace>"; fi
  done
fi
if [ "$CHECK" = 1 ]; then echo "[ok] preflight passed; nothing written"; exit 0; fi

# ---- install ----

mkdir -p "$PREFIX" || die 1 "cannot create $PREFIX"
if [ -n "$BIN" ]; then SRC=$BIN
else
  (cd "$REPO" && go build -trimpath -o "bin/apex$EXE" ./cmd/apex) || die 1 "go build failed"
  SRC=$REPO/bin/apex$EXE
fi
cp "$SRC" "$BIN_DEST" || die 1 "cannot copy apex to $BIN_DEST"
chmod +x "$BIN_DEST" 2>/dev/null
OLD_BIN=$(mf_get bin)
if [ -n "$OLD_BIN" ] && [ "$OLD_BIN" != "$BIN_DEST" ] && [ -f "$OLD_BIN" ]; then rm -f "$OLD_BIN"; fi

# Sweep where the previous install put skills as well as where this one does, so switching
# workflow or harness leaves nothing stale. Unmarked skills are never touched.
while IFS=$TAB read -r h d; do remove_marked "$d"; done <<EOF
$(prev_rows)
$TARGETS
EOF
while IFS=$TAB read -r h d; do
  mkdir -p "$d" || die 1 "cannot create $d"
  for s in $SKILLS; do
    cp -R "$REPO/skills/$s" "$d/$s" || die 1 "cannot copy skill $s to $d"
    : >"$d/$s/$APEX_MARK" || die 1 "cannot mark $d/$s"
  done
done <<EOF
$TARGETS
EOF

# Claude extras: touched only when asked for.
if [ "$STYLE" = yes ]; then
  mkdir -p "$(dirname "$STYLE_FILE")" && cp "$REPO/output-styles/apex.md" "$STYLE_FILE" || die 1 "cannot install $STYLE_FILE"
elif [ "$STYLE" = no ]; then remove_style; fi
if [ "$MODS" = yes ]; then
  claude plugin marketplace add "$REPO/mods" || die 1 "claude plugin marketplace add failed"
  for m in $APEX_MODS; do claude plugin install "$m@apex" || die 1 "claude plugin install $m@apex failed"; done
elif [ "$MODS" = no ]; then remove_mods; fi

# Manifest: written to a temp file and renamed, so it is never half written.
mkdir -p "$(dirname "$MANIFEST")" || die 1 "cannot create $(dirname "$MANIFEST")"
tmp=$MANIFEST.tmp.$$
{
  printf 'version\t1\nworkflow\t%s\nbin\t%s\nconfig\t%s\n' "$WORKFLOW" "$BIN_DEST" "$CONFIG_DIR"
  while IFS=$TAB read -r h d; do printf 'skills\t%s\t%s\n' "$h" "$d"; done <<EOF
$TARGETS
EOF
} >"$tmp" && mv -f "$tmp" "$MANIFEST" || die 1 "cannot write $MANIFEST"

case ":$PATH:" in *":$PREFIX:"*) ;; *) warn "$PREFIX is not on PATH; every skill calls a bare apex" ;; esac
echo "[ok] apex -> $PREFIX; workflow $WORKFLOW ($(echo $SKILLS | wc -w | tr -d ' ') skills) -> $(echo "$TARGETS" | cut -f2 | tr '\n' ' '). Restart your harness."
