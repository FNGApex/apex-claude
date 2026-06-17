# Project signals

## Framework & runtime

- Language: Go 1.26, module `apexclaude` (no external dependencies)
- Claude Code plugin artifacts: agents (`.md`), skills (`SKILL.md`), commands (`.md`), output-styles (`.md`), hooks (`hooks.json`)
- Binary: `bin/apex` (static, zero runtime deps); built with `make build`

## Build / test / lint

| Purpose | Command | Source |
|---------|---------|--------|
| Build binary | `make build` → `bin/apex` | Makefile |
| Run all tests | `make test` (go test ./...) | Makefile |
| Format Go | `make fmt` | Makefile |
| Vet Go | `make vet` | Makefile |
| Cross-compile release matrix | `make release` → `bin/<os>-<arch>/apex` | Makefile |
| Check signals freshness | `bin/apex signals stale` | internal/signals |
| Doctor check | `bin/apex doctor` | internal/doctor |
| Lint artifacts/specs | `bin/apex validate` | internal/validate |
| Doc-surface staleness | `bin/apex docs stale` | internal/docs |
| Follow-up ledger | `bin/apex followups` | internal/followups |
| Reminders | `bin/apex reminder` | internal/reminder |

Release targets: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64.

## Language breakdown

| Language | LOC | Files | % |
|----------|-----|-------|---|
| Go | 1782 | 28 | 58% |
| Markdown | 1176 | 40 | 38% |
| JSON | 62 | 4 | 2% |
| YAML | 42 | 1 | 1% |

## DevOps & CI

CI: GitHub Actions (`.github/workflows/ci.yml`), triggers on push to `main`/`master` and all pull requests.
Gate order: `gofmt` (format check on `cmd/` and `internal/`) → `go vet ./...` → `go test ./...` → `make build` → `./bin/apex doctor` → `./bin/apex validate`.
No deployment pipeline — release cross-compilation handled locally via `make release`.

---

## Domains

| Domain | Repo paths | One-liner | Detail |
|--------|------------|-----------|--------|
| backbone | cmd/apex/, internal/, go.mod, Makefile | Go CLI (`apex` v0.2.0): signals scan, health score, bash guard, session-start hook, doctor, followups, reminders, validate, docs gate | .claude/project/signals/backbone.md |
| plugin | .claude-plugin/, agents/, commands/, skills/, output-styles/, hooks/, CLAUDE.md | Claude Code plugin (`apex-claude` v0.1.0): 10 agents, 6 skills, 21 commands — full lifecycle roster (plan/implement/ship/diagnose/docs/signals/help); SessionStart + PreToolUse hooks | .claude/project/signals/plugin.md |

## Cross-cutting

- Test layout: `*_test.go` co-located with packages under `internal/`; no separate `test/` directory
- Project state files live in `.claude/project/`: `deterministic-signals.md` (scan output), `health.md` (integrity score), `doc-surfaces.md` (docs cache), `followups/` (ledger), `reminders/` (due nudges)
- Deterministic substrate: `.claude/project/deterministic-signals.md` (written by `apex signals scan`)
- Domain partitioning basis: backbone groups all Go packages + CLI dispatcher (what runs deterministically in hooks/CI); plugin groups all Claude Code artifacts (what Claude reads and interprets)
- Cross-domain coupling: `hooks/hooks.json` (plugin domain) invokes `${CLAUDE_PLUGIN_ROOT}/bin/apex hooks pre-bash` and `${CLAUDE_PLUGIN_ROOT}/bin/apex hooks session-start` (backbone domain); `internal/doctor` (backbone) validates presence of plugin artifact directories; CI runs `./bin/apex doctor` and `./bin/apex validate` as final gates
- `CLAUDE.md` at repo root carries the full Apex spine (principles, determinism boundary, lifecycle, agent/skill/command registries) plus `@.claude/project/signals.md` inside an `<atomic-signals>` block — @-ref wiring is active
