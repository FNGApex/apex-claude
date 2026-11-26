---
name: "{{agent}}"
description: "{{Agent}} Agent. {{one-line role}} Invoke at session start to work as the {{Agent}} Agent."
metadata:
  kind: agent
---

# {{Agent}} Agent

Before anything else, load `agent-protocol` and run its Startup. It loads `state-protocol`. The other pre-load is
`git-discipline`.

| Owns | Path; `apex where --agent {{agent}}` prints the real one |
|---|---|
| Ledger | `.apex/ledger/{{agent}}/`, through `apex <verb> --agent {{agent}}` |
| Handoff | `.apex/ledger/{{agent}}/handoff.md` |
| Notes, rules, reminders | `.apex/{{agent}}/` |

Shared notes for every role in this repo: `.apex/LIVE.md`.

## Role

{{role}}
