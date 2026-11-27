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

The contracts are [docs/design/orchestration.md](docs/design/orchestration.md) (Orchestration-based Workflow) and
[docs/design/agents.md](docs/design/agents.md) (shared state and legacy Multi-Agent Collaboration).

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
`scripts/install.sh`. Key flags include `--workflow` (`orchestration|multi-agent|both`), `--harness`
(`agents|claude|codex|gemini|t3|custom`), `--skills-dir`, `--prefix`, `--config-dir`, `--with-output-style`,
`--with-claude-mods`, `--check-ponytail`, `--custom` / `--interactive`, `--yes`, `--check`, and `--bin`
(`-Workflow`, `-Harness`, etc. in PowerShell; pass `--help` / `-Help` for the full list).

**Harness presets** decide where skills and the reply style go:

| Preset | Installs skills into |
|---|---|
| `agents` | the shared `.agents/skills/` convention that most harnesses read |
| `claude` | the Claude Code config directory, with its extras (below) |
| `codex` | the Codex skills directory |
| `gemini` | the Gemini / Antigravity skills directory, `~/.gemini/config/skills` |
| `t3` | T3 Code, a multi-provider harness: one run installs into `agents`, `claude`, `codex` and `gemini` |
| `custom` | a directory you name |

The installer never overwrites or removes a skill folder it did not install; it marks its own. `apex` must be on the
`PATH` of the shell your harness runs commands in, since every skill calls a bare `apex`. Restart the harness after
installing. Upgrading an older install, or switching workflow or harness: see [MIGRATION.md](MIGRATION.md), Part 3.

### T3 Code harness

T3 Code routes each delegate to one of several providers and creates child worktrees itself. Two things tie it in:

- **Tier routing.** `.apex/tiers.md` maps each capability tier to a T3 Code provider and model id (the T3 Code
  harness lists them with `orchestrator_capabilities`), so `audit` can run on a different provider than `deep`.
- **Worktrees.** The Orchestrator delegates with the T3 Code harness's `isolation: "worktree"`, then registers the
  child's worktree or branch with `apex worker adopt`. From there `apex worker merge` and `apex worker prune` work as
  for a spawned worker.

### Claude Code extras

Optional, and only for Claude Code:

- `mods/`: Claude Code plugins installed as `<mod>@apex` from the local `apex` marketplace. `token-spend` adds
  `/spend` (token and dollar spend); `usage-footer` adds a footer with what is left of each limit.
- `output-styles/apex.md`: the `reply-style` skill as a Claude Code output style. Pick it with `/output-style Apex`.
- `ponytail`: an optional plugin for the `lean` lens of `/pr-review`. Without it the lens walks its own checklist.

Then adopt the protocol in a repo with one session that runs `/apex-init`. A repo that already holds prompts,
personas, task lists, harness memory or a paned Multi-Agent Collaboration setup is moved over by
[MIGRATION.md](MIGRATION.md).

## Repository layout

An adopted repo holds its state under `.apex/` and its skills under `.agents/skills/`:

```text
<repo>/
  AGENTS.md                     entry point for the harness: where Apex state and skills live
  .apex/
    LIVE.md                     shared notes: facts true for everyone working in this repo
    git.md                      this repo's commit, push and PR conventions
    tiers.md                    which model runs each capability tier
    ledger/orchestrator/        NNN.md items (never deleted), handoff.md, concurrent marker
    orchestrator/               private to the checkout: LIVE.md, RULES.md, reminder skills
    workers/                    git-ignored: <id>.json manifests and <id>/ temporary worktrees
  .agents/skills/
    orchestrator/SKILL.md       the Orchestrator's role card
    <starter or domain>/        worker skills: one folder per kind of work
```

### Legacy `.claude/` layout

Repos adopted before this layout keep everything under `.claude/` and keep working: the `apex` CLI finds either layout
(`apex where` prints which). [MIGRATION.md](MIGRATION.md) covers every move: an ad-hoc or foreign AI repo onto Apex
Orchestration, a legacy paned Multi-Agent Collaboration setup (coexist, consolidate, or move the layout only), and
machine installs.

```text
<repo>/.claude/
  skills/<agent>/SKILL.md       role card of a legacy Agent
  apex/<agent>/                 private to the checkout: LIVE.md, RULES.md, reminder skills
  ledger/<agent>/NNN.md         one ledger per Agent, with handoff.md
  LIVE.md, git.md, tiers.md     shared notes, git conventions, tier map
```

## The `apex` CLI

All verbs run in the repo and resolve state paths through `apex where`. Every ledger verb takes `--agent <name>`
(`--agent orchestrator` in the Orchestration-based Workflow, or a named Agent in Multi-Agent Collaboration).

### Ledger

| Verb | Does |
|---|---|
| `add <task\|followup> <title>` | files a new open item, prints its path |
| `list [--status s,s\|all] [--kind k]` | lists items; default status `in-progress,open` |
| `start <id>` | open to in-progress |
| `close <id> <reason>` | to closed |
| `reopen <id>` | in-progress or closed to open |
| `retitle <id> <title>` | replaces an item's title, any status |
| `where [--agent <a>] [--json]` | prints where the repo's Apex state lives; `--agent` narrows it to one Agent |

### Workers

Repo-scoped verbs for managing stateless temporary worker worktrees (all accept `--json`):

| Verb | Does |
|---|---|
| `worker spawn <slug> [--base b] [--skill a,b] [--item ref]` | allocates an id, creates branch `apex/<id>-<slug>`, worktree `.apex/workers/<id>/`, and `<id>.json` |
| `worker adopt <worktree-or-branch> [--slug s] [--base b] [--skill a,b] [--item ref]` | registers an external harness worktree or branch (e.g. T3 `isolation: "worktree"`) under `.apex/workers/<id>.json` |
| `worker list [--state s,s\|all]` | lists workers with live git status (`worktree`, `ahead`, `behind`, `dirty`) |
| `worker status <id>` | shows one worker's manifest and live git status |
| `worker merge <id> [--message m]` | `--no-ff` merges a worker's branch into `base` (exits `5` on conflict) |
| `worker remove <id> [--force] [--purge]` | removes an Apex-spawned worktree and branch, or untracks an adopted external worker (`--purge` deletes the external worktree and branch too); keeps `<id>.json` marked `removed` |
| `worker prune` | removes clean `merged` workers and prunes stale worktrees |

### Skills

Repo-scoped verbs for discovering and checking skills across repo, env, and user roots (all accept `--json`):

| Verb | Does |
|---|---|
| `skill list [--kind k] [--tag t] [--source s]` | lists discovered skills |
| `skill search <query...> [--kind k] [--tag t] [--limit n]` | ranks matching skills by name, tags, triggers, description, and domain |
| `skill show <name\|domain/name> [--body]` | prints one skill's metadata and path (`--body` appends the markdown body) |
| `skill check [<path-or-name>]` | validates skill frontmatter (`name`, `description`, `kind`, `tier`) |

### Exit codes

| Code | Means |
|---|---|
| 0 | ok |
| 1 | not found |
| 2 | environment |
| 3 | illegal transition |
| 4 | corrupt state (ledger, worker manifest, or skill frontmatter) |
| 5 | merge conflict |
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
