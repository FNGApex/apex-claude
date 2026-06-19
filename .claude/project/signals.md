# Project signals

## Framework & runtime

- Language: Go 1.26, module `apexclaude` (no external dependencies)
- Install model: loose `~/.claude/` artifacts (commands, agents, skills, output-styles, binary) — NOT a Claude Code plugin; multiple install paths (see install domain)
- Hooks wired via `~/.claude/settings.json` (PreToolUse + SessionStart) by install scripts using an embedded Python snippet; no plugin enable/disable lifecycle
- Binary: `bin/apex` (static, zero runtime deps); built with `make build`

## Build / test / lint

| Purpose | Command | Source |
|---------|---------|--------|
| Build binary | `make build` → `bin/apex` | Makefile |
| Run all tests | `make test` (go test ./...) | Makefile |
| Format Go | `make fmt` | Makefile |
| Vet Go | `make vet` | Makefile |
| Cross-compile release matrix | `make release` → `bin/<os>-<arch>/apex` | Makefile |
| Install loose artifacts to ~/.claude (source) | `make install` → `scripts/install.sh` | Makefile / scripts/install.sh |
| Install loose artifacts to ~/.claude (prebuilt, Unix) | `scripts/install-release.sh` | scripts/install-release.sh |
| Install loose artifacts to ~/.claude (prebuilt, Windows) | `scripts/install.ps1` | scripts/install.ps1 |
| Remove loose artifacts from ~/.claude (Unix) | `make uninstall` → `scripts/uninstall.sh` | Makefile / scripts/uninstall.sh |
| Remove loose artifacts from ~/.claude (Windows) | `scripts/uninstall.ps1` | scripts/uninstall.ps1 |
| Publish prebuilt release bundles (Linux/macOS) | `scripts/publish.sh` | scripts/publish.sh |
| Publish prebuilt release bundles (Windows) | `scripts/publish.ps1` | scripts/publish.ps1 |
| Check signals freshness | `bin/apex signals stale` | internal/signals |
| Doctor check | `bin/apex doctor` | internal/doctor |
| Lint artifacts/specs | `bin/apex validate` | internal/validate |
| Doc-surface staleness | `bin/apex docs stale` | internal/docs |
| Follow-up ledger | `bin/apex followups` | internal/followups |
| Reminders | `bin/apex reminder` | internal/reminder |

Release targets: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64.

**Install paths:**
1. Source build (Unix): `scripts/install.sh` — needs go + make + python3; flags: `--release`, `--no-build`, `--help`. Idempotent.
2. Prebuilt download (Linux/macOS): `scripts/install-release.sh` — downloads `apex-claude-<os>-<arch>.zip` from GitHub Releases via curl/wget; needs curl/wget + python3, NO go/make. Env overrides: `APEX_VERSION`, `CLAUDE_CONFIG_DIR`. One-liner: `curl -fsSL .../install-release.sh | bash`.
3. Prebuilt download (Windows): `scripts/install.ps1` — PowerShell; no external toolchain required.

## Language breakdown

| Language | LOC | Files | % |
|----------|-----|-------|---|
| Go | 2003 | 29 | 41% |
| Markdown | 1681 | 58 | 35% |
| Shell | 644 | 4 | 13% |
| PowerShell | 395 | 3 | 8% |
| JSON | 65 | 5 | 1% |
| YAML | 42 | 1 | 1% |

## DevOps & CI

CI: GitHub Actions (`.github/workflows/ci.yml`), triggers on push to `main`/`master` and all pull requests.
Gate order: `gofmt` (format check on `cmd/` and `internal/`) → `go vet ./...` → `go test ./...` → `make build` → `./bin/apex doctor` → `./bin/apex validate`.
No deployment pipeline — release cross-compilation and GitHub Release publishing handled locally via `scripts/publish.sh` (Linux/macOS) or `scripts/publish.ps1` (Windows). End-user install is one of three paths (see above); not CI-driven.

---

## Domains

| Domain | Repo paths | One-liner | Detail |
|--------|------------|-----------|--------|
| backbone | cmd/apex/, internal/, go.mod, Makefile | Go CLI (`apex` v0.2.0): signals scan, health score, bash guard, session-start hook, doctor, followups, reminders, validate, docs gate | .claude/project/signals/backbone.md |
| install | scripts/install.sh, scripts/install-release.sh, scripts/uninstall.sh, scripts/install.ps1, scripts/uninstall.ps1, scripts/publish.sh, scripts/publish.ps1 | Three install paths + two publish tools: (1) source build via install.sh/make (needs go+make+python3); (2) Unix prebuilt download via install-release.sh (needs curl/wget+python3); (3) Windows prebuilt via install.ps1 (PowerShell, no toolchain). publish.sh/publish.ps1 cross-compile, bundle, and upload GitHub Releases. All install paths wire PreToolUse+SessionStart hooks via embedded Python into ~/.claude/settings.json. | .claude/project/signals/install.md |
| plugin | .claude-plugin/, agents/, commands/, skills/, output-styles/, hooks/, CLAUDE.md | Claude Code artifact surface (`apex-claude` v0.2.0): 10 agents, 7 skills, 23 commands — full lifecycle roster (plan/implement/ship/diagnose/docs/signals/handoff/resume/help); hooks.json retained as reference but hooks now wired by install scripts into settings.json, not via plugin manifest | .claude/project/signals/plugin.md |

## Cross-cutting

- Test layout: `*_test.go` co-located with packages under `internal/`; no separate `test/` directory
- Project state files live in `.claude/project/`: `deterministic-signals.md` (scan output), `health.md` (integrity score), `doc-surfaces.md` (docs cache), `followups/` (ledger), `reminders/` (due nudges)
- Deterministic substrate: `.claude/project/deterministic-signals.md` (written by `apex signals scan`)
- Domain partitioning basis: backbone = Go packages + CLI (deterministic layer); install = all deploy/publish scripts (distribution mechanism); plugin = Claude Code artifact surface (what Claude reads and interprets)
- Install model: three distinct install paths now exist — source build (install.sh, needs go+make), Unix prebuilt (install-release.sh, needs only curl/wget+python3), Windows prebuilt (install.ps1, bundled PowerShell). Prior plugin manifest (`.claude-plugin/`) is retained in repo but is not the active install vehicle. Commands appear as bare `/ax-*` (not `/apex-claude:ax-*`).
- Publish tooling: two maintainer-facing publish scripts (publish.sh for Linux/macOS, publish.ps1 for Windows) cross-compile the full release matrix, bundle each platform as `apex-claude-<os>-<arch>.zip`, and upload to a GitHub Release via `gh`; release notes include both the Unix curl one-liner and the Windows irm one-liner.
- Cross-domain coupling: all install scripts (install domain) build/bundle from backbone and copy plugin artifacts; hooks wired via Python into `~/.claude/settings.json` reference `~/.claude/bin/apex` (the installed backbone binary). `internal/doctor` (backbone) validates presence of plugin artifact directories relative to the repo root, not the install root.
- CI does not run install — CI gates are build + vet + test + doctor + validate; install/publish are local developer/maintainer operations.
- `CLAUDE.md` at repo root carries the full Apex spine (principles, determinism boundary, lifecycle, agent/skill/command registries) plus `@.claude/project/signals.md` inside an `<apex-signals>` block — @-ref wiring is active
