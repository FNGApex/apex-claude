# install

## What it does
Two idempotent bash scripts that deploy and remove Apex Claude as loose user-level artifacts in `~/.claude/` (not as a Claude Code plugin). `install.sh` is the primary deploy path; `make install` delegates to it.

Hooks (PreToolUse + SessionStart) are wired directly into `~/.claude/settings.json` via an embedded Python 3 snippet — no plugin enable/disable lifecycle. Each re-run strips prior Apex hook entries before re-inserting, so it is safe to run multiple times.

## Files
- `scripts/install.sh` — full deploy script (build → migrate → copy artifacts → install binary → wire hooks)
- `scripts/uninstall.sh` — removal script (delete ax-* artifacts + apex.md + binary; strip apex hooks from settings.json)
- `Makefile` targets `install` and `uninstall` — thin wrappers: `bash scripts/install.sh` / `bash scripts/uninstall.sh`

## install.sh flow
1. **Build** (`make build`; skip with `--no-build`; optionally `make release` with `--release`)
2. **Migrate** — if a prior plugin install of `apex-claude@apex-claude` is present, `claude plugin uninstall` it to avoid duplicate `/ax-*` and `/apex-claude:ax-*` commands
3. **Copy artifacts** — `commands/ax-*.md` → `~/.claude/commands/`; `agents/ax-*.md` → `~/.claude/agents/`; `skills/ax-*/` → `~/.claude/skills/`; `output-styles/protocol.md` → `~/.claude/output-styles/apex.md`
4. **Binary** — `bin/apex` → `~/.claude/bin/apex` (chmod +x)
5. **Wire hooks** — Python 3 merges PreToolUse(Bash) and SessionStart entries into `~/.claude/settings.json`, referencing `~/.claude/bin/apex`; all other settings preserved

## uninstall.sh flow
- Removes `~/.claude/commands/ax-*.md`, `~/.claude/agents/ax-*.md`, `~/.claude/skills/ax-*`, `~/.claude/output-styles/apex.md`, `~/.claude/bin/apex`
- Strips apex hook entries from `~/.claude/settings.json` (Python 3); deletes empty hook sections; does not touch any other setting
- Does NOT remove `~/.claude/CLAUDE.md` — the Apex spine is opt-in

## Install path vs. repo path
The installed binary lives at `~/.claude/bin/apex`; the repo binary lives at `bin/apex`. Hook commands reference the installed path. After code changes, re-run `make install` to refresh the installed binary; the repo binary is updated by `make build`.

## Prerequisites
- `python3` on PATH (always required — used for settings.json merge)
- `go` and `make` on PATH (required unless `--no-build`)
- `claude` CLI on PATH only needed if migrating from a prior plugin install (step 2 is gated on `have claude`)

## Coupling
- install.sh copies artifact files from the plugin domain (commands/, agents/, skills/, output-styles/); any rename or addition in those directories requires a corresponding deploy via re-run of install.sh
- Hook commands are hardcoded to `~/.claude/bin/apex hooks pre-bash` / `~/.claude/bin/apex hooks session-start`; renaming backbone subcommands requires updating this script
- `$CLAUDE_CONFIG_DIR` env var overrides the default `~/.claude` destination (useful for testing in alternate installs)
