# plugin

## What it does
Packages a coding-workflow agent layer as a Claude Code plugin named "apex-claude" (version 0.1.0, defaultEnabled: false).
The plugin wires a PreToolUse(Bash) hook that runs `${CLAUDE_PLUGIN_ROOT}/bin/apex hooks pre-bash` before every Bash tool call.
Ten agents, two skills, one command, and one output style compose the full workflow surface.

## Artifacts
- .claude-plugin/plugin.json — plugin manifest; declares name, version, description, defaultEnabled
- agents/ax-builder.md — feature-checkpoint builder (sonnet); TDD loop; fails on broad scope with "BRIEF TOO BROAD: ..."
- agents/ax-investigator.md — read-only code locator (haiku); returns file:line table; refuses edits with "OUT OF SCOPE: ax-investigator is read-only. Route edits to ax-builder."
- agents/ax-reviewer.md — diff reviewer (sonnet); one finding per line with emoji severity; ends with exactly `VERDICT: PASS` or `VERDICT: CHANGES_REQUESTED`
- agents/ax-strategist.md — read-only heavyweight reasoning (opus); audits plans/specs/designs; answers "is this the right approach?" not "is this code correct?"
- agents/ax-signals-inferrer.md — full signals pipeline; runs apex signals scan, infers domains, authors signals.md, wires @-ref; scoped writes only
- agents/ax-git-scout.md — read-only stale-git-state scanner; inspects worktrees, local branches, remote refs; dispatched by /ax-git-cleanup
- agents/ax-haiku.md — lightweight background runner (haiku); polling, status checks, log scraping; self-contained brief; backs /ax-watch-ci
- agents/ax-debug.md — hypothesis-driven debugger; forms symptom→hypothesis→cheapest-test chain; lands failing repro test then fixes; backs /ax-diagnose
- agents/ax-plan.md — planner/large-context combiner; researches online + codebase, writes design doc + spec in own context; may dispatch any subagent except ax-builder
- agents/ax-writer.md — prose drafter for human-facing markdown; draft→expand chain (Opus 4.8 draft, Fable 5 expand); dispatched by ax-explainer skill
- commands/ax-ship.md — slash command; gates commits behind ax-reviewer verdict; delegates message format to ax-commit skill
- skills/ax-commit/SKILL.md — commit message skill; Conventional Commits `type(scope): subject`; subject ≤50 chars; allowed tools scoped to git status/diff/log only
- skills/ax-tdd/SKILL.md — TDD enforcement skill; auto-triggers on implementation phrases; failing test must precede production code; skips for pure docs/config
- output-styles/protocol.md — "Protocol" output style; signal-first, drops filler phrases; hedging only when genuinely uncertain
- hooks/hooks.json — hook wiring; PreToolUse(Bash) only; no SessionStart hook in this file

## Coupling
- Changing the ax-reviewer verdict format (`VERDICT: PASS` / `VERDICT: CHANGES_REQUESTED`) breaks the ax-ship command gate that parses it.
- Changing ax-commit skill trigger phrases or Conventional Commits format affects ax-ship, which delegates to ax-commit for the commit message.
- Adding or renaming agents requires updating any command or skill that dispatches them by name (currently ax-ship dispatches ax-reviewer).
- Adding hooks to hooks.json requires the `${CLAUDE_PLUGIN_ROOT}/bin/apex` binary to handle the new hook event; the binary is not defined inside this plugin's tracked source paths.
- Changing plugin.json `name` or `defaultEnabled` affects how the Claude Code harness loads and activates the plugin.
- ax-signals-inferrer dispatches ax-investigator per domain on large repos; renaming ax-investigator breaks that dispatch.

## Conventions worth knowing
- ax-reviewer finding format: `path:line: <emoji> severity: problem. fix.` where 🟥 = blocker, 🟧 = risk, 🟨 = nit.
- ax-builder output block ends with `## Did` and `## Signals` sections.
- ax-commit allowed-tools are restricted to `Bash(git status*)`, `Bash(git diff*)`, `Bash(git log*)` — no file reads or writes.
- ax-tdd skips are always explained inline: `skipped TDD because: <reason>`.
- Protocol output style permits prose only for security warnings, irreversible-action confirmations, and non-obvious tradeoffs.
- hooks.json references `${CLAUDE_PLUGIN_ROOT}` as an environment variable; the plugin root path is not hardcoded.
- ax-debug is opt-in only — dispatched when a test breaks or the orchestrator is stuck, not as routine implementation.
- ax-plan keeps research in its own context to avoid polluting the orchestrator's cache.
- ax-writer inverts the terse Apex output style; scope is enduring narrative docs only, not specs or signals.
