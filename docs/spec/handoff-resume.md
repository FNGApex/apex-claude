# Spec — handoff / resume

Binary-backed session continuity. `apex handoff` owns deterministic scan + staleness exit codes;
`/ax-handoff`, `/ax-resume`, and the `ax-handoff` skill compose the narrative. Design rationale and
pressure-test conclusions: `docs/design/handoff-resume.md`.

## Contract

- Single active doc: `.claude/project/handoff.md`. Frontmatter via `internal/fm`, ordered keys:
  `mode` (graceful|urgent), `created` (RFC3339), `branch`, `head` (short sha), `health`,
  `status` (open|consumed).
- On consume, archive to `.claude/project/handoffs/<id>.md`, `id` = `%03d` next over that dir
  (mirrors followups CLOSED archive).
- On `handoff write`, if an un-consumed active doc already exists, **archive it first** (same
  archive path) before writing the new one — nothing is silently lost.
- `apex handoff scan` is ONE scanner serving both capture (input to handoff.md) and recovery
  (rescan with no doc). Captures: git branch / HEAD / dirty / staged, last commit, open followups,
  due reminders, health score, signals staleness, active scratchpad `BRIEF.md`.
- `apex handoff status` exit codes: `0` present+fresh, `2` present+stale (live HEAD != recorded
  `head`), `1` absent. Branch drift = human-readable note, not a code.
- Two modes — graceful sections: Shipped / Outcome / Next / Open threads; urgent sections:
  Cursor / Uncommitted / Resume here / Blockers.
- Auto-fire phrases object-bearing only; bare "resume" excluded. Skill auto-fire confirms before
  destructive ops; explicit commands skip confirm.

## Checkpoints

| # | Checkpoint | File(s) | Verify |
|---|---|---|---|
| 1 | `internal/handoff` package: `State` struct + `Scan(root) (State, error)` capturing git branch/HEAD/dirty/staged + last commit via isolated `git*` helpers | `internal/handoff/handoff.go` | `go test ./internal/handoff` — Scan populates git fields in a temp repo |
| 2 | Scan extends to non-git sources: open followups, due reminders, health score, signals staleness, active `BRIEF.md` (library calls, not re-shell) | `internal/handoff/handoff.go` | unit test: seeded state dir → Scan returns counts/score |
| 3 | `Render(State, mode, now time.Time) string` emits the mode's section skeleton + `internal/fm` frontmatter (`mode/created/branch/head/health/status`); `now` injected for a deterministic RFC3339 `created` | `internal/handoff/handoff.go` | test: graceful vs urgent render distinct sections; fm round-trips |
| 4 | `Path(root)`, `Write(root, State, mode, now)` (active doc; if an un-consumed active doc exists, Archive it first), `Status(root) int` (exit 0/1/2 per staleness), `Archive(root) (id, error)` (move active → `handoffs/<id>.md`, set `status: consumed`) | `internal/handoff/handoff.go` | tests: Status returns 1 absent / 0 fresh / 2 after HEAD change; Archive moves file + assigns `%03d`; Write over an existing active doc archives the old one first |
| 5 | `cmd/apex/cmd_handoff.go`: `register("handoff",…)` → `switch args[0]` over `scan\|status\|archive`, exit-code returns, usage on unknown sub | `cmd/apex/cmd_handoff.go` | `go build ./...`; `apex handoff status` exits 1 in a repo with no handoff |
| 6 | `commands/ax-handoff.md` (argument-hint `[graceful|urgent]`): scan → draft mode sections → write active doc | `commands/ax-handoff.md` | `apex validate` passes; `/ax-handoff` listed |
| 7 | `commands/ax-resume.md`: `handoff status` → route 0/1/2 → reconcile read-only vs live state (`git status`, followups, health) → confirm → `handoff archive` on accept. `handoff scan` is reserved for route-1 reconstruct only (it writes — must not overwrite a doc being consumed) | `commands/ax-resume.md` | `apex validate` passes |
| 8 | `skills/ax-handoff/SKILL.md`: scoped auto-fire phrases (object-bearing; bare "resume" excluded), destructive-confirm guardrail on auto-fire | `skills/ax-handoff/SKILL.md` | `apex validate` passes; skill discoverable |
| 9 | Roster wiring: add handoff/resume to CLAUDE.md command roster + `.claude/project/signals.md` plugin signals | `CLAUDE.md`, `.claude/project/signals.md` | grep shows entries; `apex validate` clean |
| 10 | Full gate: `make fmt && make vet && make test && make build && ./bin/apex doctor && ./bin/apex validate` all green | — | all commands exit 0 |

Doctor/validate scan command + skill dirs by glob — no code edit needed there for checkpoints 6–8.

## Change log

- 2026-06-19 — Initial spec. Strategy B (binary-backed). Staleness = sha-inequality → exit 2
  (ancestry math rejected). Auto-fire phrases object-bearing, bare "resume" excluded.
- 2026-06-19 — Approval-gate decisions locked: `handoff write` archives an existing un-consumed
  active doc before writing (no silent overwrite); `head` = short sha; adopt the recommended
  object-bearing phrase additions; destructive-confirm guardrail on skill auto-fire only.
- 2026-06-19 — Checkpoints 1–4 implemented. `Render`/`Write` take an injected `now time.Time` for a
  deterministic `created` stamp (refinement during impl; body updated to current truth).
