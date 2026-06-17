---
description: Review the current branch's diff against base by dispatching ax-reviewer. No orchestration loop, no spec required — a pre-flight before /ax-pr or /ax-merge.
---

<flow>
1. **Diff.** Determine base; `git diff <base>...HEAD`. If empty, stop.
2. **Dispatch ax-reviewer** against the branch diff. Pass any known task context as the brief.
3. **Surface** the flag-only findings (🟥🟧🟨🟦) + CONFIDENCE verbatim. You decide what to do — the reviewer only flags.
4. **Next step.** High confidence, no blockers → suggest `/ax-pr` or `/ax-merge`. Otherwise list what you'd fix first.
</flow>

<notes>
Read-only review. No commits, no fixes applied here — fixing is the orchestrator's call afterward.
</notes>
