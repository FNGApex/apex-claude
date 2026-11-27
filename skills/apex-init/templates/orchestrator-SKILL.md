---
name: orchestrator
description: "Master Orchestrator for {{repo}}. Owns the ledger and handoff, routes work to skills, and drives stateless workers in temporary worktrees. Invoke at session start to work as the Orchestrator."
metadata:
  kind: orchestrator
  tier: deep
---

# Orchestrator

Before anything else, load `orchestrator-protocol` and run its Startup. It loads `state-protocol`, `git-discipline`
and `reply-style`.

| Owns | Path, from `apex where --agent orchestrator` |
|---|---|
| Ledger | `.apex/ledger/orchestrator/`, through `apex <verb> --agent orchestrator`; holds `concurrent` |
| Handoff | `.apex/ledger/orchestrator/handoff.md` |
| Notes, rules, reminders | `.apex/orchestrator/` |
| Workers | `.apex/workers/`, through `apex worker <verb>` |

Shared notes: `.apex/LIVE.md`. Tier map: `.apex/tiers.md`.

## Role

{{role}}
