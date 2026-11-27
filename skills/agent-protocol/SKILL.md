---
name: agent-protocol
description: The protocol a Collaboration Agent follows in Multi-Agent Collaboration, the legacy Apex workflow where named Agents each own a ledger and work as peers. Loads state-protocol for startup, ledger, handoff and notes, then covers picking and working one item at a time, approach modes, review, closing, and peer messaging and filing. Pre-loaded by an Agent's skill; new setups use orchestrator-protocol instead. Use whenever a Collaboration Agent starts a session, works a ledger item or talks to a peer.
metadata:
  kind: protocol
---

# Agent protocol: Multi-Agent Collaboration (legacy workflow)

You are a Collaboration Agent: a skill with `metadata.kind: agent` that owns one ledger and one handoff and works with
peer Agents. New setups use the Orchestration-based Workflow and `orchestrator-protocol`; this protocol stays for
repos that run named peer Agents.

Load `state-protocol` first. It holds paths (`apex where --agent <agent>`), the ledger verbs and exit codes, the
markers, the item body and close prefixes, the handoff format and freshness check, and your notes. Run its Startup.

## Pick the next item

`apex list --agent <agent>`. Resume the in-progress item if there is one. Otherwise take what the operator names or the
handoff's Next. New work the operator asks for gets filed first with `apex add` and then worked like any item.

## Work an item

One item at a time, unless your ledger holds `concurrent`. Close the current one, or return it to open with
`apex reopen <id> --agent <agent>`, before starting another.

1. `apex start <id> --agent <agent>` (skip when resuming an in-progress item).
2. Fill `## Goal`: a `From:` line and checkable done-criteria, as `state-protocol` describes.
3. Fill `## Approach` with one of these and the reason:

   | Mode | When |
   |---|---|
   | direct | the change is small, or the context it needs is already in this session |
   | subagent | a self-contained slice that benefits from a fresh context |
   | worker | one change in a temporary worktree: follow the Dispatch, Watch and Merge back sections of `orchestrator-protocol` |
   | fan-out | independent slices run by parallel workers, one worktree each, as in `orchestrator-protocol` |

   Subagents and workers get the item's Goal. They report back; they own no state.
4. Append one line to `## Log` per step taken, as you go.
5. Review the result against the Goal's done-criteria. Run a review skill when the repo provides one (`apex skill
   search review`) and act on its findings; otherwise review by hand for correctness, scope and needless code, and say
   which in the Log.
6. Update your notes when you learned something durable, per `state-protocol`. Skip it when nothing durable came up.
7. Write `## Outcome`.
8. `apex close <id> "done: <one line>" --agent <agent>`.
9. Run the commit check in `git-discipline`, silently. When it passes, commit the item's work: the code plus the
   pending state-directory changes. In a worktree, the code commits in the worktree and the state-directory changes
   commit in `<root>`. When it fails, commit nothing.
10. Rewrite the handoff after step 9, so `head` covers any commit made there; State names work left uncommitted.

## Collaboration

Peers talk through the harness's peer-messaging primitive: one call finds running peers, another sends a message.

- To give a peer work, file it into the peer's ledger. Get the peer's home repo path from the peer or from a
  `LIVE.md`, then run `cd <peer home repo> && apex add <kind> <title> --agent <peer>` as one command; `apex` has no
  directory flag. Write `From: <your repo>:<you>/<your item id>` in its Goal, then message the peer if it is running.
- When a peer's message asks you for something, what you do depends on `apex list --agent <agent>`:

  | Your state | Do |
  |---|---|
  | no item in progress | act on the request and reply right away |
  | an item in progress | `apex add <kind> <title> --agent <agent>`, write `From: <peer repo>:<peer>/<peer item id>` in its Goal, reply with the new item's id, and return to your item |

  A message that announces an item the peer already filed in your ledger is not filed again.
- Peer traffic stays out of the operator's chat. Tell the operator in one line that a peer's message arrived and was
  answered or filed; never repeat to the operator what you sent to a peer. Put the content in front of the operator
  only when you need the operator's answer.
- Only the owner starts or closes an item. A peer only files: it never runs `start`, `close`, `reopen` or `retitle`
  with another Agent's name.

### Claude Code harness

`ListAgents` finds running peers and `SendMessage` reaches them.
