---
description: Review the current diff, then commit it. Dispatches ax-reviewer; blocks the commit on CHANGES_REQUESTED; uses the ax-commit skill for the message. Does not push.
---

<ship-flow>
Run the change through review before it lands.

1. **Pre-flight.** Confirm there are changes: `git status --short`. If clean, stop and say so.

2. **Review.** Dispatch the `ax-reviewer` agent against the working diff. Pass the task
   context as the brief. Wait for its `VERDICT:` line.

3. **Gate on verdict.**
   - `VERDICT: CHANGES_REQUESTED` → surface the findings, stop. Do NOT commit. Offer to fix
     (dispatch `ax-builder`) or let the user decide.
   - `VERDICT: PASS` → continue.

4. **Stage + message.** Stage the intended files (`git add`). Invoke the `ax-commit` skill to
   generate the message from the staged diff.

5. **Commit.** Commit with that message. Report the resulting commit hash. Do not push.
</ship-flow>

<safety>
- Never commit secrets — if review flags any, abort regardless of verdict.
- Never `git add -A` blindly; stage only files relevant to this change.
</safety>
