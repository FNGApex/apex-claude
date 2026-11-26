---
name: sign-off
description: End a stateful Apex session cleanly, for the Orchestrator and for Collaboration Agents. Logs or closes in-progress ledger items, updates notes, audits worker worktrees, writes the handoff, cleans up what the session started, commits the session's state-directory changes, and reports unpushed commits. Use on /sign-off or when the operator says "sign off".
metadata:
  kind: protocol
---

# Sign off

The operator's notice that the session ends. Run the seven steps in order, as the role this session was started as:
the Orchestrator or a Collaboration Agent. Load `state-protocol`; ledger, handoff, notes and paths come from it and
from `apex where --agent <agent>`. Every stateful `apex` call passes `--agent <agent>`.

## 1. In-progress items

Find them with `apex list --status in-progress --agent <agent>`. None: skip to step 2.

| Item state | Action |
|---|---|
| Goal met and reviewed | write `## Outcome`, `apex close <id> "done: <one line>" --agent <agent>`, run the `git-discipline` commit check |
| Not done | append one `## Log` line saying where it stopped and what comes next; leave it in progress |

## 2. Notes

Update the notes `state-protocol` defines with what this session learned that a future session needs: a state fact to
`<private>/LIVE.md`, a trap or a do or don't to a reminder skill there, a rule the operator decided to
`<private>/RULES.md`, a fact every role needs to `<state>/LIVE.md`, once. Remove notes the session proved wrong.

## 3. Worker worktree audit

Run `apex worker list`. No workers: skip to step 4.

| Worker | Action |
|---|---|
| `merged`, clean | removed by `apex worker prune` |
| `active` with `ahead > 0` (unmerged commits) or `dirty > 0` | report id, branch, item, `ahead` and `dirty`; ask the operator: merge back, keep for the next session, or remove. Never remove one without that answer |
| `active`, clean, `ahead == 0` | report it; remove with `apex worker remove <id>` if this session spawned it and its item is closed, else keep |
| `worktree` `missing` or `drift` | report it; change nothing |

Run `apex worker prune` once the merged ones are known. Every worker still active goes into the handoff in step 4.
A Collaboration Agent that spawned no workers sees an empty list and moves on.

## 4. Handoff

Overwrite the handoff in the format `state-protocol` defines. Take `checkout`, `branch` and `head` from the checkout
worked in at this moment. State names the items from step 1, any uncommitted work outside the state directory, and
each worker left active: id, branch, item, and why it stays.

## 5. Clean up what the session started

Only what this session created or launched. Anything that predates the session stays.

| Kind | What counts | How |
|---|---|---|
| Scratch files | temp files and directories the session created | `rm` the paths |
| Servers | dev servers, watchers, background tasks the session started and that still run | stop the background task, or `kill <pid>` |
| Other worktrees | worktrees the session created outside `apex worker`, whose branch is merged into `<base>` (`git branch --merged <base>`) | `git worktree remove <path>`, then `git branch -d <branch>` |

`<base>` is the branch checked out in `<root>`. Worker worktrees are handled in step 3 only. Unmerged worktrees and
uncommitted changes are reported, never removed.

## 6. Commit the state directory

Commit the session's state-directory changes in `<root>` per `git-discipline`: on the operator's approval, or
automatically when `<state>/git.md` says so. The commit lands on whatever branch `<root>` has checked out and stages
only the state-directory paths this session changed, never the operator's other staged or unstaged work. Private
folders and `<state>/workers/` ignore themselves and are never staged.

## 7. Unpushed commits

For each checkout the session committed in, other than worker worktrees:

```sh
git log --oneline HEAD --not --remotes   # unpushed commits
git rev-parse --abbrev-ref @{u}          # upstream name; fails when there is none
```

Report branch, upstream (or "no upstream") and the commits. Temporary `apex/` worker branches are never reported:
they are never pushed. Push only if the operator says so, following `git-discipline`.
