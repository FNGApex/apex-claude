---
description: Schedule a reminder. Creates a reminder via apex reminder and schedules it (cron < 1h, Routines >= 1h). Degrades to file-only when scheduling tools are unavailable.
argument-hint: <timing> <what to remember>
---

<flow>
1. **Parse** timing + message from $ARGUMENTS (e.g. "after the PR", "tomorrow", "in 2h").
2. **Create** the reminder: `apex reminder add` with the message and resolved due time.
3. **Schedule.** < 1h → cron; >= 1h → Routines. If neither transport is available, leave it file-only and say so — `apex reminder due` will surface it at session start.
4. **Confirm** what was scheduled and when it fires.
</flow>
