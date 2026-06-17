---
description: Bootstrap the current repo for Apex. Audits .gitignore, docs/ layout, CLAUDE.md presence, and signals wiring. Proposes only what's missing — never overwrites. No commits.
---

<flow>
1. **Audit.** Check for: `CLAUDE.md` (with a signals `@-ref`), `docs/design/` + `docs/spec/`, `.claude/project/` state dir, gitignore entries for regenerable caches (`bin/`, `deterministic-signals.md`, `health.md`, `doc-surfaces.md`, `.claude/.scratchpad/`, `tmp/`, `.worktrees/`).
2. **Report gaps.** Table: surface · present? · proposed action. Propose only missing pieces.
3. **Apply on approval.** Create missing dirs/files. Never overwrite existing user content.
4. **Signals.** If signals absent, run `/ax-refresh-signals` (initializes on first run).
5. **Verify.** `apex doctor`. Report remaining gaps. No commit — leave staging to the user.
</flow>
