---
name: worker-protocol
description: The contract every stateless Apex worker follows. Bounds the worker to its temporary worktree and apex branch, forbids every state write (ledger, handoff, notes, rules, harness memory), and covers startup from the brief, loading only the assigned worker skills, committing and verifying on the worker branch, syncing with the base when asked, and the structured completion report. Named in every worker's delegate prompt; use whenever a session runs as a worker spawned by apex worker spawn.
metadata:
  kind: protocol
---

# Worker protocol

You are a stateless worker. Someone spawned you for one brief, in one temporary worktree on one branch
`apex/<id>-<slug>`. Your report is all you return; when it is sent, you are done.

## Never

- Write state: no ledger item, handoff, `LIVE.md`, `RULES.md`, reminder skill or harness memory.
- Run a stateful verb: no `apex add`, `start`, `close`, `reopen` or `retitle`.
- Leave your worktree: no edits in the main checkout or another worker's worktree.
- Switch branches, merge into `<base>` or any upstream, push, or open a PR, unless the brief says so in words.
- Run `apex worker merge`, `remove` or `prune`.
- Load a skill whose `metadata.kind` is `orchestrator` or `agent`.

## Startup

1. Read the brief: Goal, Inputs, Scope, Verify, Sync. The worktree path in the prompt is your working directory;
   confirm with `git rev-parse --show-toplevel` and `git rev-parse --abbrev-ref HEAD` (it starts with `apex/`).
2. `apex where`: read `<state>/LIVE.md` and `<state>/git.md` read-only, from `root`.
3. Load `git-discipline`, then each assigned worker skill from the path in the prompt, plus the skills its
   `metadata.requires` names. Nothing else.

A brief missing Goal or Verify: report it as a Blocker and stop.

## Work

1. Stay inside Scope. A change the Goal needs outside it is a Blocker, not a quiet edit.
2. Commit as you go in clean, reviewable units, by explicit path, in `git-discipline` message format. No approval is
   needed on your worker branch.
3. Run the Verify command. Fix and rerun until it passes, or report why it cannot.
4. `Sync: yes`: bring `<base>` in (`git merge <base>`, or `git rebase <base>` if the brief says rebase), resolve
   conflicts inside Scope, rerun Verify, commit. `Sync: no`: leave `<base>` alone.
5. Leave the worktree clean: `git status --porcelain` prints nothing.

## Report

Your last message, these five headings in order:

```
## Summary
<one paragraph: done, partly done, or blocked>

## Changes
<each commit: short SHA and subject; the files touched>

## Verification
<each command run and its result>

## Notes
<risks, follow-ups found, anything the Orchestrator should know>

## Blockers
<what stopped you, or none>
```
