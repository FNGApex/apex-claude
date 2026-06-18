# Project signals

## Framework & runtime

- Language: Go 1.26, module `apexclaude` (no external dependencies)
- Install model: loose `~/.claude/` artifacts (commands, agents, skills, output-styles, binary) — NOT a Claude Code plugin; `scripts/install.sh` deploys, `scripts/uninstall.sh` removes
- Hooks wired via `~/.claude/settings.json` (PreToolUse + SessionStart) by install.sh using an embedded Python snippet; no plugin enable/disable lifecycle
- Binary: `bin/apex` (static, zero runtime deps); built with `make build`

## Build / test / lint

| Purpose | Command | Source |
|---------|---------|--------|
| Build binary | `make build` → `bin/apex` | Makefile |
| Run all tests | `make test` (go test ./...) | Makefile |
| Format Go | `make fmt` | Makefile |
| Vet Go | `make vet` | Makefile |
| Cross-compile release matrix | `make release` → `bin/<os>-<arch>/apex` | Makefile |
| Install loose artifacts to ~/.claude | `make install` → `scripts/install.sh` | Makefile / scripts/install.sh |
| Remove loose artifacts from ~/.claude | `make uninstall` → `scripts/uninstall.sh` | Makefile / scripts/uninstall.sh |
| Check signals freshness | `bin/apex signals stale` | internal/signals |
| Doctor check | `bin/apex doctor` | internal/doctor |
| Lint artifacts/specs | `bin/apex validate` | internal/validate |
| Doc-surface staleness | `bin/apex docs stale` | internal/docs |
| Follow-up ledger | `bin/apex followups` | internal/followups |
| Reminders | `bin/apex reminder` | internal/reminder |

Release targets: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64.

`install.sh` flags: `--release` (build cross-compile matrix first), `--no-build` (skip build), `--help`. Requires `python3` at install time (settings.json merge); requires `go` + `make` unless `--no-build`. Idempotent and safe to re-run.

## Language breakdown

| Language | LOC | Files | % |
|----------|-----|-------|---|
| Go | 1782 | 28 | 48% |
| Markdown | 1583 | 56 | 43% |
| Shell | 216 | 2 | 6% |
| JSON | 65 | 5 | 2% |
| YAML | — | 1 | <1% |

## DevOps & CI

CI: GitHub Actions (`.github/workflows/ci.yml`), triggers on push to `main`/`master` and all pull requests.
Gate order: `gofmt` (format check on `cmd/` and `internal/`) → `go vet ./...` → `go test ./...` → `make build` → `./bin/apex doctor` → `./bin/apex validate`.
No deployment pipeline — release cross-compilation handled locally via `make release`. End-user install is `make install` (loose artifacts) or manual copy; not CI-driven.

---

## Domains

| Domain | Repo paths | One-liner | Detail |
|--------|------------|-----------|--------|
| backbone | cmd/apex/, internal/, go.mod, Makefile | Go CLI (`apex` v0.2.0): signals scan, health score, bash guard, session-start hook, doctor, followups, reminders, validate, docs gate | .claude/project/signals/backbone.md |
| install | scripts/install.sh, scripts/uninstall.sh | Bash deploy scripts: build apex, copy loose artifacts into ~/.claude/{commands,agents,skills,output-styles,bin}, wire PreToolUse + SessionStart hooks into ~/.claude/settings.json via embedded Python; uninstall reverses all steps | .claude/project/signals/install.md |
| plugin | .claude-plugin/, agents/, commands/, skills/, output-styles/, hooks/, CLAUDE.md | Claude Code artifact surface (`apex-claude` v0.1.0): 10 agents, 6 skills, 21 commands — full lifecycle roster (plan/implement/ship/diagnose/docs/signals/help); hooks.json retained as reference but hooks now wired by install.sh into settings.json, not via plugin manifest | .claude/project/signals/plugin.md |

## Cross-cutting

- Test layout: `*_test.go` co-located with packages under `internal/`; no separate `test/` directory
- Project state files live in `.claude/project/`: `deterministic-signals.md` (scan output), `health.md` (integrity score), `doc-surfaces.md` (docs cache), `followups/` (ledger), `reminders/` (due nudges)
- Deterministic substrate: `.claude/project/deterministic-signals.md` (written by `apex signals scan`)
- Domain partitioning basis: backbone = Go packages + CLI (deterministic layer); install = bash deploy scripts (distribution mechanism); plugin = Claude Code artifact surface (what Claude reads and interprets)
- Install model shift: prior model installed via `claude plugin install` using `.claude-plugin/plugin.json`; current model copies artifacts directly into `~/.claude/` as user-level loose files via `scripts/install.sh`. Plugin manifest (`.claude-plugin/`) is retained in repo but is not the active install vehicle. Commands appear as bare `/ax-*` (not `/apex-claude:ax-*`).
- Cross-domain coupling: `scripts/install.sh` (install domain) builds from backbone and copies plugin artifacts; hooks wired via Python into `~/.claude/settings.json` reference `~/.claude/bin/apex` (the installed backbone binary). `internal/doctor` (backbone) validates presence of plugin artifact directories relative to the repo root, not the install root.
- CI does not run install — CI gates are build + vet + test + doctor + validate; install is a local developer operation.
- `CLAUDE.md` at repo root carries the full Apex spine (principles, determinism boundary, lifecycle, agent/skill/command registries) plus `@.claude/project/signals.md` inside an `<apex-signals>` block — @-ref wiring is active
