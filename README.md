# apex

A skill set and a small CLI for running coding agents on a repo, for any model and any coding harness. The `apex`
binary owns the deterministic part (ledger, workers, skill index); everything else is plain markdown skills that any
harness that reads skills can load.

Two workflows ship:

- **Orchestration-based Workflow** (default). One stateful Master Orchestrator keeps the ledger, plans nothing it
  cannot file as an item, and hands each item to a stateless worker. A worker is a skill-based agent that runs in a
  temporary worktree of its own, reports back, and is merged or discarded. The Orchestrator is the only thing that
  holds state between sessions.
- **Multi-Agent Collaboration** (legacy). User-defined Agents, each with its own ledger, handoff and private notes,
  working one item at a time and messaging peers. It is kept for repos that already run it.

The contract is [docs/design/agents.md](docs/design/agents.md).

## Install

Requires Go 1.26+ and `git`.

```sh
git clone git@github.com:FNGApex/apex-claude.git && cd apex-claude
make install                  # apex, plus the skills of the default workflow, into your harness
make install-orchestration    # the Orchestration-based Workflow
make install-multi-agent      # the Multi-Agent Collaboration (legacy) workflow
make install-custom           # choose the workflow and the harness interactively
make uninstall
```

On Windows, without `make` (PowerShell 5.1 or 7+): `pwsh scripts/install.ps1`. On Linux and macOS, without `make`:
`scripts/install.sh`. Both take `--help` (`-Help` in PowerShell) for the flags that pick the workflow, the harness
preset and the install locations.

**Harness presets** decide where skills and the reply style go:

| Preset | Installs skills into |
|---|---|
| `agents` | the shared `.agents/skills/` convention that most harnesses read |
| `claude` | the Claude Code config directory, with its extras (below) |
| `codex` | the Codex skills directory |
| `custom` | a directory you name |

The installer never overwrites or removes a skill folder it did not install; it marks its own. `apex` must be on the
`PATH` of the shell your harness runs commands in, since every skill calls a bare `apex`. Restart the harness after
installing.

### Claude Code extras

Optional, and only for Claude Code:

- `mods/`: Claude Code plugins installed as `<mod>@apex` from the local `apex` marketplace. `token-spend` adds
  `/spend` (token and dollar spend); `usage-footer` adds a footer with what is left of each limit.
- `output-styles/apex.md`: the `reply-style` skill as a Claude Code output style. Pick it with `/output-style Apex`.
- `ponytail`: an optional plugin for the `lean` lens of `/pr-review`. Without it the lens walks its own checklist.

Then adopt the protocol in a repo with one session that runs `/apex-init`.

## Repository layout

An adopted repo holds its state under `.apex/` and its skills under `.agents/skills/`:

```text
<repo>/
  AGENTS.md                     entry point for the harness: where Apex state and skills live
  .apex/
    LIVE.md                     shared notes: facts true for everyone working in this repo
    git.md                      this repo's commit, push and PR conventions
    tiers.md                    which model runs each capability tier
    ledger/NNN.md               one item per file, never deleted
    orchestrator/               the Orchestrator's handoff and private notes
    workers/<id>/               one worker: its worktree, branch record and report
  .agents/skills/
    orchestrator/SKILL.md       the Orchestrator's role card
    <starter or domain>/        worker skills: one folder per kind of work
```

### Legacy `.claude/` layout

Repos adopted before this layout keep everything under `.claude/` and keep working: the `apex` CLI finds either layout
(`apex where` prints which). [MIGRATION.md](MIGRATION.md) moves a repo to the new layout.

```text
<repo>/.claude/
  skills/<agent>/SKILL.md       role card of a legacy Agent
  apex/<agent>/                 private to the checkout: LIVE.md, RULES.md, reminder skills
  ledger/<agent>/NNN.md         one ledger per Agent, with handoff.md
  LIVE.md, git.md, tiers.md     shared notes, git conventions, tier map
```

## The `apex` CLI

All verbs run in the repo and read its state through `apex where`.

### Ledger

| Verb | Does |
|---|---|
| `add <task\|followup> <title>` | files a new open item, prints its path |
| `list [--status s,s\|all] [--kind k]` | lists items; default status `in-progress,open` |
| `start <id>` | open to in-progress |
| `close <id> <reason>` | to closed |
| `reopen <id>` | in-progress or closed to open |
| `retitle <id> <title>` | replaces an item's title, any status |
| `where [--agent <a>] [--json]` | prints where the repo's Apex state lives; `--agent` narrows it to one legacy Agent |

In the legacy layout every ledger verb takes `--agent <name>`.

### Workers

| Verb | Does |
|---|---|
| `worker spawn <id> <slug>` | creates a temporary worktree and branch for a worker on a ledger item |
| `worker list` | lists workers |
| `worker status <id>` | shows one worker: state, branch, unmerged commits |
| `worker merge <id>` | merges a finished worker's branch |
| `worker remove <id>` | removes a worker's worktree and record |
| `worker prune` | removes the workers that are merged and finished |

### Skills

| Verb | Does |
|---|---|
| `skill list` | lists the skills of the repo |
| `skill search <query>` | finds a skill by its name or description |
| `skill show <name>` | prints one skill |
| `skill check` | validates every skill's frontmatter |

### Exit codes

| Code | Means |
|---|---|
| 0 | ok |
| 1 | not found |
| 2 | environment |
| 3 | illegal transition |
| 4 | corrupt ledger |
| 5 | conflict |
| 64 | usage |

## Skills

| Skill | Use |
|---|---|
| `orchestrator-protocol` | the Orchestrator's session: startup, the item loop, handing items to workers |
| `worker-protocol` | a worker's session: take one item, work it in its worktree, report |
| `state-protocol` | the shared rules for `.apex/`: ledger, handoff, notes |
| `reply-style` | terse, answer-first replies, for any model |
| `apex-init` | adopts the protocol in a repo: initialize, migrate, or report conformant |
| `git-discipline` | commit, push and PR rules, and the repo's conventions from `git.md` |
| `sign-off` | ends a session: log, notes, handoff, cleanup, unpushed commits |
| `pr-review` | `/pr-review <pr> [general\|lean\|security\|mega]`: reviews one pull request through a one-off workflow and serves the draft review before posting; `ponytail` is an alias for `lean` |
| `agent-protocol` | the Multi-Agent Collaboration (legacy) protocol every Agent pre-loads |

## Capability tiers

Skills and workflows name a capability tier, never a model. `tiers.md` maps each tier to the model your harness runs
for it; a blank tier runs on the session's own model.

| Tier | For |
|---|---|
| `fast` | cheap, mechanical work: search, formatting, small edits |
| `standard` | everyday implementation and review |
| `deep` | hard reasoning: judging, planning, editing |
| `audit` | an independent second opinion on settled work |
| `security` | security review; has no fallback, so it needs a model mapped |
