---
description: Capture a session handoff so a fresh-context session can resume the work. Binary scans the deterministic state; you compose the narrative. Two modes — graceful (clean stop) | urgent (limits/time).
argument-hint: [graceful | urgent]
---

<flow>
The binary owns the deterministic capture and the staleness anchor; you own the intent narrative.

1. **Mode.** `graceful` (default) when stopping at a clean boundary; `urgent` when cut short by
   context/time. Pass through as the argument.

2. **Scan + skeleton.** Run `apex handoff scan [mode]`. This captures git branch/HEAD/dirty/staged,
   last commit, open followups, due reminders, health, signals staleness, and any active scratchpad
   `BRIEF.md`, then writes `.claude/project/handoff.md` with frontmatter + the mode's empty section
   skeleton. If an un-consumed handoff already exists, the binary archives it first — nothing is lost.

3. **Compose.** Edit `.claude/project/handoff.md`, filling the mode's sections from THIS session —
   the part the binary can't know:
   - graceful → **Shipped** (what landed) / **Outcome** (decisions, locked choices) /
     **Next** (the very next action) / **Open threads** (loose ends).
   - urgent → **Cursor** (exact stopping point) / **Uncommitted** (in-flight edits) /
     **Resume here** (first action on return) / **Blockers**.
   Be concrete: name files, commands, decisions. Convert relative dates to absolute. Do NOT touch
   the frontmatter — `head` is the staleness anchor.

4. **Report.** Print the doc path and a one-line summary of what's captured.
</flow>

<notes>
- This is the capture verb. To consume a handoff, use `/ax-resume`.
- The doc stays `status: open` until `/ax-resume` archives it on accept.
</notes>
