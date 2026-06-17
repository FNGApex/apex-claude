# backbone

## What it does
Go CLI binary named `apex` (version 0.2.0) with four subcommands (signals, health, hooks, doctor) that implement the deterministic safety and introspection layer for the Claude Code plugin.
Internal packages write project state to `.claude/project/` and enforce bash guardrails via PreToolUse hooks; the model never touches this layer directly.
Module is `apexclaude` (go 1.26), built to `bin/apex` via `make build`; cross-compiled for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64 by `make release`.

## CLI code
- cmd/apex/main.go — entry point; declares binary name `apex` and version `0.2.0`
- cmd/apex/registry.go — `commands` map populated by each cmd_*.go `init()` function
- cmd/apex/cmd_signals.go — registers `signals` with sub-subcommands: scan, show, stale; delegates to internal/signals and internal/proj
- cmd/apex/cmd_health.go — registers `health` with sub-subcommands: show, set <0-100> [note]; delegates to internal/health and internal/proj
- cmd/apex/cmd_hooks.go — registers `hooks` with sub-subcommands: pre-bash, session-start; pre-bash → guard.PreBash, session-start → hooks.SessionStart
- cmd/apex/cmd_doctor.go — registers `doctor`; delegates to doctor.Run
- internal/signals/signals.go — Scan() fingerprints manifests + top-level non-dotted dirs via SHA-256 and writes .claude/project/deterministic-signals.md; Stale() exits 0 (fresh), 1 (stale/missing), 2 (error); Show() returns file contents
- internal/guard/guard.go — Evaluate() blocks: rm -rf on root/home/$HOME paths, force-push to main/master (--force-with-lease is allowed), curl/wget pipe to sh/bash; PreBash() reads JSON hook payload from stdin and writes deny JSON to stdout on block, exits 2
- internal/health/health.go — Show() returns score (-1 if unset) and file body; Set() writes score 0-100 with optional note to .claude/project/health.md using `apex-health-score: <N>` HTML comment as the machine-readable anchor
- internal/hooks/hooks.go — SessionStart() calls signals.Stale(); emits additionalContext nudge JSON if stale; always returns 0 (never blocks session start)
- internal/doctor/doctor.go — Run() validates: .claude-plugin/plugin.json valid JSON, hooks/hooks.json valid JSON, output-styles/ has .md, agents/ has .md, commands/ has .md, skills/ has a subdirectory containing SKILL.md (glob: skills/*/SKILL.md); reports signals freshness as info; resolves plugin root from $CLAUDE_PLUGIN_ROOT or binary location (bin/apex → repo root)
- internal/proj/proj.go — Root() returns $APEX_REPO if set, else cwd; StateDir() returns <root>/.claude/project, creating it if absent
- go.mod — module apexclaude, go 1.26
- Makefile — targets: build, test (go test ./...), fmt, vet, clean, release (cross-compile matrix); install is declared in .PHONY but has no recipe

## Coupling
- Changing the `.claude/project/deterministic-signals.md` format (written by internal/signals) requires updating Stale() fingerprint logic and any consumer that reads this file (e.g. the session-start hook and doctor info check)
- Changing the `apex-health-score: <N>` HTML comment pattern in internal/health requires updating any tooling or skill that parses health.md
- Adding a new subcommand requires a new cmd_*.go with an `init()` that registers into the `commands` map in registry.go
- guard.Evaluate() block rules are the authoritative list of blocked bash patterns; any change here affects what Claude Code can run without a deny response
- $APEX_REPO and $CLAUDE_PLUGIN_ROOT are the two env-var seams that let the CLI operate outside the canonical repo layout (CI, remote envs, tests)

## Conventions worth knowing
- Subcommands self-register via init() — adding a file is enough; no manual wiring in main.go
- PreBash hook communicates block decisions exclusively through stdout JSON + exit code 2; non-zero exit with no deny JSON is an error condition, not a block
- SessionStart never blocks (always exits 0); it can only emit advisory context
- Stale() exit codes are semantic: 0 = fresh, 1 = stale or file missing, 2 = error — callers must distinguish 1 from 2
- Fingerprint covers manifest files (go.mod, package.json, Cargo.toml, etc.) hashed by name+size (not content) and non-dotted top-level dirs by name only; dotfiles, file content, and nested dirs are excluded by design
- doctor.Run() treats signals freshness as info, not a hard failure — all other checks are hard failures
