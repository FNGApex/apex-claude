---
description: Refresh project signals on demand (initializes on first run). Dispatches the ax-signals-inferrer agent to scan, infer domains, write signals.md, and wire the @-ref into CLAUDE.md.
---

<flow>
1. **Staleness check.** `apex signals stale`. Exit 0 (fresh) → report and stop unless the user forces. Exit 1 (stale) → refresh. Exit 2 (no baseline) → first run, proceed.
2. **Dispatch ax-signals-inferrer.** It runs `apex signals scan`, infers domains, authors `.claude/project/signals.md`, and wires `@.claude/project/signals.md` into CLAUDE.md if absent.
3. **Verify.** Confirm `apex signals stale` exits 0. Stage `deterministic-signals.md` + `signals.md` (and per-domain files) if the user is committing.
</flow>

<notes>
The deterministic scan is authoritative — the agent never hand-edits `deterministic-signals.md`, only regenerates it. Ship verbs dispatch this silently when signals go stale.
</notes>
