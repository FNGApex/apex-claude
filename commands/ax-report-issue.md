---
description: Open a GitHub issue via gh for the current project. Bug report or feature request — auto-detect from the description. Structures the body with repro/expected/actual (bug) or problem/proposal (feature).
argument-hint: <description>
---

<flow>
1. **Classify.** Bug vs feature from $ARGUMENTS (error/broken/regression → bug; want/add/support → feature).
2. **Gather context.** For a bug: relevant `apex version`, OS, repro steps, expected vs actual. For a feature: the problem and proposed behavior.
3. **Draft body.** Bug: Repro / Expected / Actual / Environment. Feature: Problem / Proposal / Alternatives. Signal-first, no filler.
4. **Confirm** the title + body with the user (opening an issue is outward-facing).
5. **Open.** `gh issue create --title <t> --body <b>`. Report the URL.
</flow>

<safety>
Opening an issue publishes externally. Confirm content first; never include secrets or internal paths that shouldn't be public.
</safety>
