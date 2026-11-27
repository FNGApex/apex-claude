---
name: orchestrator-protocol
description: The protocol the Master Orchestrator follows in the Orchestration-based Workflow. Covers startup and worker reconciliation, routing work across hundreds of skills with apex skill search and show, spawning stateless workers in temporary worktrees with apex worker spawn (or adopting a harness-created one with apex worker adopt), writing briefs, watching and verifying workers, merge-back on request or policy with conflict handling, and cleanup. Pre-loaded by the Orchestrator's skill; use whenever the Orchestrator starts a session, routes or dispatches work, verifies a worker or merges one back.
metadata:
  kind: protocol
---

# Orchestrator protocol

You are the Orchestrator: the one stateful role. You own the ledger and the handoff, route work to skills, and drive
stateless workers. You do not do a worker's job yourself unless the item's Approach says `direct`. `<agent>` is
`orchestrator` unless your skill says otherwise. The design is in `docs/design/orchestration.md` of the Apex repo.

Load `state-protocol`, `git-discipline` and `reply-style`. Replies follow `reply-style`.

## Startup

1. `apex where --agent <agent>`: note `root`, `state`, `workers`, `tiers`.
2. Run the `state-protocol` Startup, steps 2-6.
3. Read `<state>/tiers.md`: the model each capability tier maps to in this harness.
4. `apex worker list`. Reconcile it against the handoff's State: a worker the handoff names but `list` does not, a
   worker `list` shows that the handoff does not name, and any `worktree` that is `missing` or `drift`. Report each.
5. `apex list --agent <agent>`.
6. Report in a few lines: handoff fresh or stale, items in progress and open, live workers with their state, and what
   comes next.

## Pick and approach

Take the item the operator names, else the handoff's Next. New work is filed first with `apex add`. The ledger holds
`concurrent`, so several items may be in progress; keep each one's Log current.

Fill `## Approach`:

| Mode | When |
|---|---|
| direct | small, or the context is already in this session; you do it in `<root>` or your worktree |
| subagent | a read-only question that needs a fresh context and no worktree |
| worker | one change in its own worktree, by one worker |
| fan-out | independent slices of one item, one worker each, run in parallel |

## Route to skills

A repo may hold hundreds of skills. Learn about them in three levels and stop at the first that answers.

1. **Find.** `apex skill search <words from the Goal> --kind worker-skill --limit 10`, or
   `apex skill list --kind worker-skill --tag <t>` when you know the tag.
2. **Decide.** `apex skill show <name>` for each candidate: tier, inputs, outputs, verify, requires, path.
3. **Use.** The worker loads the body from that path. You never load it.

Never read `.agents/skills/` or any skill root yourself, and never list every description into your context. Pass
`--body` to `apex skill show` only when no worker can run and you must follow the skill yourself. No skill fits: say so
in the Log and either brief a worker with no skill or ask the operator.

## Dispatch a worker

1. `apex start <id> --agent <agent>`, unless it is already in progress.
2. Pick the path. Your harness either delegates into a working directory you name (Path A) or creates the child
   worktree itself (Path B, the T3 Code harness). Use the one the harness primitives table in
   `docs/design/orchestration.md` lists for it, and never both for one worker.

   **Path A, CLI-managed worktree.** Spawn, then delegate into that worktree path:

   ```bash
   apex worker spawn <slug> --skill <a,b> --item <agent>/<id> [--base <branch>] --json
   ```

   It prints `id`, `worktree`, `branch` (`apex/<wid>-<slug>`) and `base`. `--base` defaults to the branch checked out
   where you run it.

   **Path B, harness-isolated worktrees (the T3 Code harness).** Call `orchestrator_capabilities` once per thread,
   then, on that harness, dispatch `delegate_task` with `isolation: "worktree"`, taking `providerInstanceId`, `model`
   and `options` from the tier's row in `<state>/tiers.md`. Wait for the completion wakeup; do not poll. Then register
   what the harness created, before you run Watch and verify or Merge back:

   ```bash
   apex worker adopt <worktree-or-branch> --skill <a,b> --item <agent>/<id> [--base <branch>] --json
   ```

   It prints the same `id`, `worktree`, `branch` and `base`, and from then on every `apex worker` verb treats the
   worker like a spawned one.
3. Write the brief, inline in the delegate prompt or as `<workers>/<wid>.brief.md` (git-ignored with the rest of
   `<workers>/`):

   ```
   ## Goal      From: <agent>/<id>; what done means, checkable
   ## Inputs    files, prior reports, links
   ## Scope     paths it may change; what it must leave alone
   ## Verify    the command that proves done, run in the worktree
   ## Sync      yes | no
   ```

4. Delegate with the harness's delegate primitive (background delegate for fan-out), with the model for the skill's
   `metadata.tier` from `<state>/tiers.md`. The prompt names: the worktree path as working directory (Path A) or
   that the harness assigns it (Path B), the brief, `worker-protocol`, and each assigned skill's path from
   `apex skill show`.
5. Log one line: worker id, branch, skills, tier.

On Path A never use the harness's own worktree isolation: `apex worker spawn` owns the worktree. At most 10 workers are live at
once unless a `RULES.md` line raises the cap; `apex worker list --state active` counts them.

## Watch and verify

While workers run, `apex worker list` and `apex worker status <wid>` show `ahead`, `behind`, `dirty` and the worktree
state. Do not poll in a loop: the delegate's completion is the signal.

On each report:

1. Read it: Summary, Changes, Verification, Notes, Blockers.
2. Check the commits yourself:

   ```bash
   git -C <wt> log --oneline <base>..HEAD
   git -C <wt> diff --stat <base>...HEAD
   ```

3. Rerun the brief's Verify command in `<wt>`. A large or risky diff gets a `review` worker of its own on the same
   branch, read-only.
4. Log the verdict. Failed: re-dispatch to the same worker worktree with what failed, or drop the worker.
   Follow-ups from Notes are filed with `apex add followup`.

## Merge back

Only when the operator asks, or when `<state>/git.md` says `Worker merge: after verification` or a `RULES.md` line
allows it, and only after verification passed.

```bash
apex worker merge <wid> [--message "<type>(<scope>): <summary>"]
```

It needs `dirty == 0`, `ahead > 0` and a clean index in the checkout holding `<base>`, and makes a `--no-ff` merge
there. The message follows `git-discipline`.

**Exit 5, conflict.** The merge was aborted and the conflicting files listed. Never resolve in the base checkout.
Re-dispatch a sync step to that worker's worktree: merge or rebase `<base>`, resolve, rerun Verify, commit. Then
retry `apex worker merge <wid>`.

## Clean up and close

- A merged worker: `apex worker prune` removes every clean merged one.
- A worker whose work is dropped: `apex worker remove <wid>`. With commits or changes still in it, ask the operator
  before `--force`.
- An adopted worker's worktree and branch belong to the harness: `remove` and `prune` only untrack it and leave them
  in place. Pass `--purge` to `apex worker remove` only when the operator wants the external worktree and branch
  deleted as well.
- When every worker of an item is merged or dropped and the Goal is met: write `## Outcome`, `apex close <id> "done:
  <one line>" --agent <agent>`, run the `git-discipline` commit check, then rewrite the handoff.

## Peers

A Collaboration Agent in this repo or another is reached through the operator or by filing into its ledger
(`apex add <kind> <title> --agent <peer>` run in the peer's home repo) with `From: <repo>:<agent>/<id>`. You never run
`start`, `close`, `reopen` or `retitle` with another role's name.
