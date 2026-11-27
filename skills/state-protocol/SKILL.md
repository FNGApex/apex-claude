---
name: state-protocol
description: The state rules every stateful Apex role shares, the Orchestrator and Collaboration Agents alike. Covers resolving paths with apex where, the apex ledger verbs and exit codes, the per-ticket and concurrent markers, item bodies and close reasons, the handoff format and freshness check, and upkeep of private notes, rules, reminder skills and the shared LIVE.md. Loaded by orchestrator-protocol and agent-protocol; use whenever a stateful role starts a session, works a ledger item or writes its handoff.
metadata:
  kind: protocol
---

# State protocol

You are a stateful role: an Orchestrator or a Collaboration Agent. You own one ledger and one handoff. `<agent>` below
is your skill's directory name (`orchestrator` for the Orchestrator). Workers and subagents never follow this skill:
they are stateless and run no stateful verb.

## Paths

Never derive a path by hand. Run:

```bash
apex where --agent <agent>
```

| Key | Is |
|---|---|
| `root` | the main checkout, also when you work in a worktree |
| `state` | the state directory, `<state>` below: `.apex`, or `.claude` in a legacy layout |
| `skill` | your skill file; user-owned, never edit it |
| `ledger` | your ledger directory, `<ledger>` below |
| `handoff` | your handoff file |
| `private` | your private folder, `<private>` below; its `.gitignore` holds `*`, so nothing in it is tracked |
| `live`, `git`, `tiers` | `<state>/LIVE.md`, `<state>/git.md`, `<state>/tiers.md` |

`--json` prints the same keys as one object.

## apex ledger verbs

Every stateful call passes `--agent <agent>` explicitly. There is no default role.

| Call | Does |
|---|---|
| `apex add <task\|followup> <title> --agent <a>` | files a new open item, prints its path |
| `apex list [--status s,s\|all] [--kind task\|followup] --agent <a>` | lists items; default status `in-progress,open`. The summary line counts every status for the selected `--kind`; `--status` filters only the rows. A per-ticket ledger first prints `ledger: <dir>` |
| `apex start <id> --agent <a>` | open to in-progress, prints the item path |
| `apex close <id> <reason> --agent <a>` | to closed, from open or in-progress, prints the item path |
| `apex reopen <id> --agent <a>` | in-progress or closed back to open, prints the item path |
| `apex retitle <id> <title> --agent <a>` | replaces the item's title in any status, prints the item path |

| Exit | Meaning | Response |
|---|---|---|
| 0 | ok | continue |
| 1 | not found | check the id and the `--agent` value |
| 2 | environment | read the message, stop and report |
| 3 | illegal transition | read the message; usually another item is already in progress |
| 4 | corrupt state | stop and report the file; never hand-edit frontmatter to fix it |
| 5 | merge conflict | only from `apex worker merge`; see `orchestrator-protocol` |
| 64 | usage | fix the call |

### Markers

Two empty files in `<ledger>` change how the verbs behave. Only the operator creates or deletes them, or `apex-init`
on the operator's word.

| Marker | Effect |
|---|---|
| `per-ticket` | every verb works on the ledger of the ticket the current branch names (`ENG-42` from `feat/eng-42-x`, else the branch name with `/` as `-`). Each ticket ledger has its own items from 001 and its own handoff, and `apex list` prints it on a `ledger: <dir>` line. A detached HEAD or outside git gives the mono ledger |
| `concurrent` | lifts the one-in-progress limit, so several items may be in progress at once |

## Startup

1. Run `apex where --agent <agent>`.
2. Read `<private>/LIVE.md`, `<private>/RULES.md` and every `<private>/skills/*.md` with file reads. A missing file is
   empty. A tracked `LIVE.md` next to your skill file means you are not migrated: tell the operator to run
   `/apex-init`.
3. Read `<state>/LIVE.md` with a file read, if it exists.
4. Load every skill your role's skill lists.
5. Read the handoff. If it does not exist, skip to step 7.
6. Check freshness. The handoff is stale when any of these holds:

   | Check | Command |
   |---|---|
   | recorded `checkout` no longer exists | `test -d <checkout>` |
   | `branch` differs | `git -C <checkout> rev-parse --abbrev-ref HEAD` |
   | code outside the state directory differs from `head` | `git -C <checkout> diff --quiet <head> HEAD -- . ':(exclude)<state>'` exits non-zero |

   An unknown `head` makes the diff exit 128 with `fatal: bad object`: that is stale, not a failed command. A stale
   handoff is still read. Staleness means State may be wrong: reconcile it against what changed
   (`git diff --stat <head> HEAD`, `git status`) before trusting it.
7. Run `apex list --agent <agent>`, with or without a handoff.

## Item body

The binary owns the frontmatter. You edit only the body sections. Fix a wrong title with `apex retitle`, and a wrong
close reason with `apex reopen` then `apex close`.

```
## Goal       From: <user | parent id such as 003 | repo:role/004>; done-criteria checkable by running or reading
## Approach   direct | subagent | worker | fan-out, and why
## Log        one line per step, as you go; the crash cursor
## Outcome    written at close
```

The Log's last line tells you where to resume after an interruption. Cite another role's item in full:
`<repo>:<role>/NNN`, where `<repo>` is its home repo's directory name.

Every close reason starts with one of these prefixes:

| Prefix | When |
|---|---|
| `done: ` | the Goal is met |
| `dropped: ` | the item will not be done; say why |
| `split: <ids>` | the item was replaced by children |

- **Drop:** close straight from open: `apex close <id> "dropped: <why>" --agent <agent>`.
- **Split:** `apex add` each child with a `From: <parent id>` Goal line, then
  `apex close <parent> "split: <ids>" --agent <agent>`, such as `split: 008-010` or `split: 008, 011`.
- **Other repos:** state stays in your home repo. Note the other repo's path and what changed there in the Log.

## Handoff

Overwrite the handoff after every close and whenever you stop mid-item. Fill the frontmatter by running the commands
in the checkout you worked in; `head` is always the full SHA. The keys and the four headings are literal; replace each
`<...>`.

```
---
checkout: <absolute path of the checkout worked in>
branch: <output of git rev-parse --abbrev-ref HEAD>
head: <output of git rev-parse HEAD>
---
## Startup
invoke the role's skill (reads its private folder and <state>/LIVE.md); apex list --agent <agent>

## State
<current items and where they stopped; uncommitted work; live workers with id, branch and item>

## Next
<the item to take next, and why>

## Open threads
<questions and waits that outlive the current item>
```

The Startup line is literal apart from `<state>` and `<agent>`.

## Notes

Your notes replace harness memory. Keep them current and short: rewrite or delete stale entries instead of appending
contradictions. They hold what outlives one item; per-item progress belongs in the item's Log.

| File | Holds | Who edits |
|---|---|---|
| `<state>/LIVE.md` | facts every role in this repo needs: build steps, repo conventions, shared traps | any stateful role |
| `<private>/LIVE.md` | facts about the current state that only your role needs | you |
| `<private>/RULES.md` | what the operator has decided you may do | you, only from the operator's words |
| `<private>/skills/<topic>.md` | reminders on how to do your work | you |

A fact goes in one file, never both. Move it to `<state>/LIVE.md` once a second role needs it.

**Rules.** Each line starts with `Allowed:` and grants one specific action: what, on what, when. No date, no history,
no prohibition, no trap; a "don't" goes in a reminder skill. Each rule covers a kind of work, not one item. Add,
change or delete a rule only when the operator decides it. What no rule allows needs the operator's approval.

**Reminder skills.** One topic per file, lines starting `Do:`, `Don't:`, `Trap:` or `Good to know:`. An operator limit,
such as one heavy process at a time, is a reminder, not a rule. A reminder never overrides your skill or this
protocol. It is a plain file you read, not a discoverable skill.
