# plugin

## What it does
Provides the Claude Code artifact surface for Apex: 10 agents, 6 skills, 21 commands, and one output style composing the full workflow lifecycle (plan/implement/ship/diagnose/docs/signals/help).

Installed as **loose user-level artifacts** into `~/.claude/` by `scripts/install.sh` (not as a Claude Code plugin). Commands appear as bare `/ax-*`. The `.claude-plugin/plugin.json` manifest is retained in the repo for reference but is not the active install vehicle.

Hooks (PreToolUse(Bash) + SessionStart) are wired into `~/.claude/settings.json` by `install.sh`; they invoke `~/.claude/bin/apex hooks pre-bash` and `~/.claude/bin/apex hooks session-start`.

## Artifacts
- .claude-plugin/plugin.json — plugin manifest; declares name, version, description, defaultEnabled
- .claude-plugin/marketplace.json — marketplace listing metadata
- agents/ax-builder.md — feature-checkpoint builder (sonnet); TDD loop; fails on broad scope with "BRIEF TOO BROAD: ..."
- agents/ax-investigator.md — read-only code locator (haiku); returns file:line table; refuses edits with "OUT OF SCOPE: ax-investigator is read-only. Route edits to ax-builder."
- agents/ax-reviewer.md — diff reviewer (sonnet); one finding per line with emoji severity; ends with `CONFIDENCE: <0-100>`; emits `VERDICT: PASS` or `VERDICT: CHANGES_REQUESTED`
- agents/ax-strategist.md — read-only heavyweight reasoning (opus); audits plans/specs/designs; answers "is this the right approach?" not "is this code correct?"
- agents/ax-signals-inferrer.md — full signals pipeline; runs apex signals scan, infers domains, authors signals.md, wires @-ref; scoped writes only
- agents/ax-git-scout.md — read-only stale-git-state scanner; inspects worktrees, local branches, remote refs; dispatched by /ax-git-cleanup
- agents/ax-haiku.md — lightweight background runner (haiku); polling, status checks, log scraping; self-contained brief; backs /ax-watch-ci
- agents/ax-debug.md — hypothesis-driven debugger; forms symptom→hypothesis→cheapest-test chain; lands failing repro test then fixes; backs /ax-diagnose
- agents/ax-plan.md — planner/large-context combiner; researches online + codebase, writes design doc + spec in own context; may dispatch any subagent except ax-builder
- agents/ax-writer.md — prose drafter for human-facing markdown; draft→expand chain (Opus 4.8 draft, Fable 5 expand); dispatched by ax-explainer skill
- commands/ax-setup.md — bootstrap current repo for Apex; audits .gitignore, docs/ layout, CLAUDE.md, signals wiring; proposes only what's missing; no commits
- commands/ax-plan.md — dispatches ax-plan agent to research and write design doc + checkpoint spec; trivial work gets inline spec; human approves before implementation
- commands/ax-pressure-test.md — pre-implementation gate; stress-tests a plan or spec for hidden assumptions and edge cases
- commands/ax-implement.md — orchestrates implement→review subagent loop; reads approved spec; dispatches ax-builder; gates each iteration on ax-reviewer CONFIDENCE; commits per green iteration; syncs docs
- commands/ax-diagnose.md — failure-driven work; dispatches ax-debug for hypothesis-driven root-cause then fix
- commands/ax-autopilot.md — autonomous end-to-end: plan → implement loop → ship hands-off; one human decision (how to merge)
- commands/ax-ship.md — review current diff, commit; dispatches ax-reviewer; gates commit on CONFIDENCE; uses ax-commit skill; does not push
- commands/ax-push.md — ship family: commit + push to remote
- commands/ax-pr.md — ship family: commit + push + open pull request
- commands/ax-merge.md — ship family: commit + merge into base branch
- commands/ax-squash.md — ship family: squash-merge into base branch
- commands/ax-documentation.md — diff-driven doc update; maintenance mode (ship verbs run it automatically) or on-demand
- commands/ax-refresh-signals.md — idempotent signals refresh; dispatches ax-signals-inferrer
- commands/ax-git-cleanup.md — stale git-state cleanup; dispatches ax-git-scout for candidates
- commands/ax-watch-ci.md — polls CI status; dispatches ax-haiku; surfaces failures to orchestrator
- commands/ax-review-branch.md — reviews full branch diff against spec; dispatches ax-reviewer
- commands/ax-follow-up.md — follow-up ledger management (list, add, close, triage)
- commands/ax-remind-me.md — surfaces due reminders from .claude/project/reminders/
- commands/ax-improve.md — retrospective; surfaces improvement opportunities after a task
- commands/ax-help.md — router for "which verb for my situation?"; lists all commands with descriptions
- commands/ax-report-issue.md — files a structured issue report (bug, nit, question) to the follow-ups ledger
- skills/ax-commit/SKILL.md — commit message skill; Conventional Commits `type(scope): subject`; subject ≤50 chars; allowed tools scoped to git status/diff/log only
- skills/ax-tdd/SKILL.md — TDD enforcement skill; auto-triggers on implementation phrases; failing test must precede production code; skips for pure docs/config
- skills/ax-verify/SKILL.md — evidence-before-claim gate; auto-triggers on "done"/"fixed"/"passing"/"complete" etc.; blocks claim until tool call proves it
- skills/ax-review/SKILL.md — compressed code-review comments; one line per finding (location + problem); emits CONFIDENCE 0-100; flag-only, no fixes
- skills/ax-documentation/SKILL.md — diff-driven documentation classifier; reads indexed doc surfaces; emits proposed edits; maintenance vs. bootstrap modes
- skills/ax-explainer/SKILL.md — voice module for enduring human-facing narrative (README, docs/guides, CHANGELOG); inverts terse Apex style; dispatches ax-writer
- output-styles/protocol.md — "Protocol" output style; signal-first, drops filler phrases; hedging only when genuinely uncertain; tiers: lite/full/ultra
- hooks/hooks.json — hook wiring reference; PreToolUse(Bash) and SessionStart; retained for documentation but hooks are now wired via ~/.claude/settings.json by scripts/install.sh (not loaded from this file by the plugin system)

## Coupling
- Changing the ax-reviewer verdict format (`VERDICT: PASS` / `VERDICT: CHANGES_REQUESTED` / `CONFIDENCE: <N>`) breaks ax-ship, ax-implement, and ax-review-branch which parse those tokens.
- Changing ax-commit skill trigger phrases or Conventional Commits format affects ax-ship, which delegates to ax-commit for the commit message.
- Adding or renaming agents requires updating any command or skill that dispatches them by name (ax-ship dispatches ax-reviewer; ax-implement dispatches ax-builder + ax-reviewer; ax-autopilot dispatches ax-plan agent + ax-builder + ax-reviewer).
- Adding hook events requires updating both `hooks/hooks.json` (reference) and `scripts/install.sh` (the Python snippet that writes to settings.json); the backbone binary must also handle the new event.
- Changing plugin.json `name` or `defaultEnabled` no longer affects runtime behavior (plugin is not the install vehicle); these fields are reference only.
- ax-signals-inferrer dispatches ax-investigator per domain on large repos; renaming ax-investigator breaks that dispatch.
- CLAUDE.md in this repo carries the full Apex spine (principles, lifecycle, registries); it is not auto-deployed by install.sh — users must opt in by copying the spine to their project's CLAUDE.md.

## Conventions worth knowing
- ax-reviewer finding format: `path:line: <emoji> severity: problem. fix.` where 🟥 = blocker, 🟧 = risk, 🟨 = nit, 🟦 = uncertain.
- ax-reviewer always ends output with `CONFIDENCE: <0-100>`; orchestrator aggregates confidence into `apex health`.
- ax-builder output block ends with `## Did` and `## Signals` sections.
- ax-commit allowed-tools are restricted to `Bash(git status*)`, `Bash(git diff*)`, `Bash(git log*)` — no file reads or writes.
- ax-tdd skips are always explained inline: `skipped TDD because: <reason>`.
- Protocol output style permits prose only for security warnings, irreversible-action confirmations, and non-obvious tradeoffs.
- hooks.json is a reference document; the active hook wiring is in ~/.claude/settings.json, written by scripts/install.sh with the absolute path to ~/.claude/bin/apex.
- ax-debug is opt-in only — dispatched when a test breaks or the orchestrator is stuck, not as routine implementation.
- ax-plan keeps research in its own context to avoid polluting the orchestrator's cache.
- ax-writer inverts the terse Apex output style; scope is enduring narrative docs only, not specs or signals.
- ax-verify blocks "done" claims unless a tool call in the same turn proves correctness; hedging ("should work", "looks good") is treated as an unverified claim.
