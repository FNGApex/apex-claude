---
description: Review and act on pending follow-ups and reminders. Bare invocation lists open follow-ups and due reminders; /ax-follow-up review walks stale entries for per-item disposition (extend/close/promote/skip).
argument-hint: [review | due <id>]
---

<flow>
1. **List.** `apex followups list` + `apex reminder list` (and `apex reminder due` for fired ones). Present as one indexed list.
2. **Due mode** (`due <id>`): surface that specific reminder and wait for a response.
3. **Review mode** (`review`): walk stale follow-up entries one at a time — extend (`apex reminder`/re-date), close (`apex followups close <id>`), promote (turn a plan into a spec via `/ax-plan`), or skip.
4. **Act** per the user's choice; confirm each disposition.
</flow>

<notes>
Two follow-up kinds: `finding` (review/cleanup loose ends, subject to staleness) and `plan` (deferred work, exempt from staleness). Plans render first as a backlog.
</notes>
