---
description: Route a lost user to the right Apex verb, skill, agent, or backbone subcommand. Bare invocation reads git state and recommends one next step. A topic keyword or freeform intent gives a focused pointer. tour gives a guided walkthrough.
argument-hint: [topic | intent | tour]
---

<flow>
- **Bare:** read `git status` + recent commits + open follow-ups; recommend ONE next step (e.g. "uncommitted work → /ax-ship", "stale signals → /ax-refresh-signals").
- **Topic/intent** ($ARGUMENTS): point to the matching verb. Map intents → verbs:
  - plan / design → `/ax-plan` (challenge first: `/ax-pressure-test`)
  - build from spec → `/ax-implement` · autonomous → `/ax-autopilot`
  - failure / bug → `/ax-diagnose`
  - land it → ship family: `/ax-ship` `/ax-push` `/ax-pr` `/ax-merge` `/ax-squash`
  - docs → `/ax-documentation` · signals → `/ax-refresh-signals`
  - git hygiene → `/ax-git-cleanup` · CI → `/ax-watch-ci`
  - retrospective → `/ax-improve` · reminders → `/ax-follow-up` `/ax-remind-me`
- **tour:** brief guided walkthrough of the plan→implement→ship→improve lifecycle.
</flow>

<boundary>
Router only — points to the right verb, never duplicates its behavior.
</boundary>
