# Migration

How to move a repo, a team and a machine onto Apex Orchestration, from wherever they start. Pick a target in the
overview, then follow the one part that matches your starting point.

| Starting point | Read |
|---|---|
| A repo with scattered prompts, personas, task lists or harness memory, and no Apex | [Part 1](#part-1-any-ad-hoc-or-foreign-ai-repo-to-apex-orchestration) |
| A repo running paned Multi-Agent Collaboration under `.claude/` | [Part 2](#part-2-paned-multi-agent-collaboration-to-orchestration) |
| A machine with an old `apex` install, or a team adopting T3 Code or several harnesses | [Part 3](#part-3-machine-installs-t3-code-and-multi-harness-setups) |
| A legacy repo that stays under `.claude/` but still keeps notes next to its role cards | [Part 4](#part-4-legacy-reference-per-agent-private-folders-claude-layout) |
| Anything, after the move | [Part 5](#part-5-post-migration-verification-and-troubleshooting) |

`/apex-init` automates Parts 1, 2 and 4: it inspects the repo, shows one plan, and changes nothing before the operator
approves it. Every part also lists the manual steps, which are what `/apex-init` does.

## Overview and decision matrix

### 1. Pick the target workflow

| Target | What it is | Pick it when |
|---|---|---|
| **Orchestration-based Workflow** (recommended default) | one stateful Master Orchestrator (`orchestrator`, state in `.apex/`, role card in `.agents/skills/`) that coordinates stateless skill-based workers in temporary worktrees | a new adoption, or any repo where one ledger and many short-lived workers fit |
| **Phased Coexistence** | legacy peer Agents keep working in `.claude/` while the Master Orchestrator is stood up in `.apex/` | Agents have work in progress that cannot stop; you move one Agent at a time |
| **Multi-Agent Collaboration** (legacy) | named peer Agents, each with its own ledger, handoff and notes; optionally moved from `.claude/` to `.apex/` + `.agents/skills/` | the team wants to keep paned peers and only needs a layout any harness can read |

### 2. Pick the target harness and worktree model

Apex names capability tiers and primitives, never a model or a harness tool. The harness decides four things:

| Harness | Install preset | User skill path | Framing file | Tier mapping in `.apex/tiers.md` | Worker worktree flow |
|---|---|---|---|---|---|
| Generic `.agents` | `agents` | `~/.agents/skills` | `AGENTS.md` | the `Other` column, or `default` | `apex worker spawn` |
| T3 Code | `t3` | multi-root: the `agents`, `claude`, `codex` and `gemini` paths below | `AGENTS.md`, plus the adapter of each provider it routes to | T3 provider and model ids, from `orchestrator_capabilities` | T3 Code `isolation: "worktree"`, then `apex worker adopt` |
| Claude Code | `claude` | `${CLAUDE_CONFIG_DIR:-~/.claude}/skills` | `CLAUDE.md` holding `@AGENTS.md` | model family aliases | `apex worker spawn` |
| Codex | `codex` | `${CODEX_HOME:-~/.codex}/skills` | `AGENTS.md` | model ids, or `default` | `apex worker spawn` |
| Gemini / Antigravity | `gemini` | `${GEMINI_CONFIG_DIR:-~/.gemini/config}/skills` | `GEMINI.md` holding `@AGENTS.md` | model ids, or `default` | `apex worker spawn` |

Two worktree models exist, and a repo may use both:

- **Apex-owned** (`apex worker spawn`): `apex` creates the branch `apex/<id>-<slug>` and the worktree
  `.apex/workers/<id>/`, and the harness delegate is pointed at that path. This is the default everywhere.
- **Harness-owned, adopted** (`apex worker adopt`): the harness creates the child worktree and branch itself, as T3
  Code does with `isolation: "worktree"`. `apex worker adopt <worktree-or-branch>` then registers it under
  `.apex/workers/<id>.json`, so verification, merge-back and cleanup are the same as for a spawned worker.

### Target layout

Every part ends at this layout, or at a subset of it:

```text
<repo>/
  AGENTS.md                              framing every harness reads; CLAUDE.md / GEMINI.md import it
  .gitattributes                         /.apex/** text eol=lf
  .apex/
    LIVE.md  git.md  tiers.md            shared notes, git conventions, tier to model map (tracked)
    ledger/orchestrator/NNN.md           ledger items, written only through apex verbs (tracked)
    ledger/orchestrator/handoff.md       the Orchestrator's handoff (tracked)
    ledger/orchestrator/concurrent       marker: several items may be in progress (tracked)
    orchestrator/                        private to the main checkout; its .gitignore holds *
      LIVE.md  RULES.md  skills/<topic>.md
    workers/                             git-ignored: <id>.json manifests, briefs, temporary worktrees
  .agents/skills/
    orchestrator/SKILL.md                metadata.kind: orchestrator
    <name>/SKILL.md                      metadata.kind: worker-skill
    <domain>/<name>/SKILL.md             the same, grouped; addressed as domain/name
```

Never derive a path by hand: `apex where [--agent <name>]` prints every one.

## Part 1: Any ad-hoc or foreign AI repo to Apex Orchestration

For a repo that has never run Apex but already holds AI working state in some form:

| You have | Examples | Becomes |
|---|---|---|
| Scattered prompts and personas | `.claude/agents/*.md`, `.claude/commands/*.md`, `.cursor/rules/*`, a monolithic `AGENTS.md`, `CLAUDE.md` or `GEMINI.md` | stateless `worker-skill`s (step 3), facts and reminders (step 4) |
| Ad-hoc task lists and foreign ledgers | `TASKS.md`, `TODO.md`, `DevLedger/`, `docs/handoff/` | items in `.apex/ledger/orchestrator/` (step 5) and one handoff |
| Harness auto-memory | `~/.claude/projects/<slug>/memory/*.md`, local memory files | `LIVE.md`, `RULES.md`, reminder skills (step 4) |

**Automated.** Install (step 1), open one session in the main checkout on a clean tree, and run `/apex-init`. Ask for
the work to be folded into the Orchestrator's ledger: by default a migrated foreign role becomes a Collaboration
Agent. It reads everything, shows one plan (roles, items, id map, memory sort, files to delete), waits for approval,
executes, and deletes old files only after you confirm the result.

**Manual.** Run in the main checkout, on a clean tree, in this order.

### Step 1. Install or upgrade the toolchain

```sh
scripts/install.sh --workflow orchestration --harness <preset>              # Linux, macOS, Git Bash
pwsh scripts/install.ps1 -Workflow orchestration -Harness <preset>          # Windows
```

`<preset>` is `agents`, `claude`, `codex`, `gemini`, `t3` or `custom` (with `--skills-dir DIR`); a comma list installs
into several. Restart the harness, then confirm `apex where` runs. [Part 3](#part-3-machine-installs-t3-code-and-multi-harness-setups)
covers upgrading an old install.

### Step 2. Establish the Master Orchestrator

Templates are in `skills/apex-init/templates/` of this repo, and in the installed `apex-init` skill.

```bash
mkdir -p .agents/skills/orchestrator .apex/orchestrator/skills .apex/ledger/orchestrator .apex/workers
printf '*\n' > .apex/orchestrator/.gitignore      # the private folder ignores itself
printf '*\n' > .apex/workers/.gitignore           # manifests, briefs and worktrees never reach git
: > .apex/ledger/orchestrator/concurrent          # several items may be in progress at once
```

| Path | Content |
|---|---|
| `.agents/skills/orchestrator/SKILL.md` | `templates/orchestrator-SKILL.md`: `name: orchestrator`, `metadata.kind: orchestrator`, `metadata.tier: deep`, and a `## Role` in your words |
| `.apex/ledger/orchestrator/concurrent` | empty marker file |
| `.apex/orchestrator/.gitignore` | one line, `*` |
| `.apex/orchestrator/LIVE.md` | `templates/LIVE.md`; the Orchestrator's own live notes |
| `.apex/orchestrator/RULES.md` | operator grants; created with the first grant (step 4) |
| `.apex/orchestrator/skills/*.md` | reminder skills, one topic per file (step 4) |

`.apex/orchestrator/` and `.apex/workers/` exist only in the main checkout and are never staged.

### Step 3. Decompose personas and commands into stateless worker skills

A persona prompt mixes several things. Only one of them is a skill:

| Line in the old persona or command | Goes to |
|---|---|
| how to do one kind of work (steps, checks, the done test) | a `worker-skill` body |
| a fact about the repo ("the API lives in `services/api`") | `.apex/LIVE.md` (step 4) |
| a permission ("you may push to staging") | `.apex/orchestrator/RULES.md` (step 4) |
| a trap or habit ("never run two migrations at once") | the skill body if every worker needs it, else a reminder skill (step 4) |
| identity, memory and session procedure ("you are Bob, remember what you did, update `TASKS.md`") | nothing: the Orchestrator, the ledger and `worker-protocol` supply it |
| a pending task | a ledger item (step 5) |

Split by kind of work, not by persona: one persona usually yields two to five skills, and a slash command usually
yields one. Each skill must be **stateless**: everything it needs arrives in the brief (Goal, Inputs, Scope, Verify,
Sync), and everything it produces leaves as commits on the worker branch plus the report.

Before, `.claude/agents/backend-dev.md`:

```markdown
You are Bob, the senior backend developer. You own services/api. Remember the endpoints you added in earlier
sessions. Add endpoints with a handler, a route and a test; run `make test-api` before you finish. Never run two
database migrations at once. You may push to the staging remote. Keep TASKS.md up to date.
```

After, `.agents/skills/backend/api-endpoint/SKILL.md`:

```markdown
---
name: api-endpoint
description: "Add or change one HTTP endpoint in services/api: handler, route and test, verified by the brief's command."
metadata:
  kind: worker-skill
  tier: standard
  tags: "backend, api, endpoint, handler, route"
  inputs: "goal, scope paths, verify command"
  outputs: "commits on the worker branch, completion report"
  verify: "make test-api passes and the worktree is clean"
  requires: "git-discipline"
---

# API endpoint

Run inside a worker that follows `worker-protocol`. Stay inside the brief's Scope.

## Steps

1. Read the neighbouring handlers in `services/api` and follow their shape.
2. Add the handler, the route and one test.
3. Run one database migration at a time.
4. Run the brief's Verify command; fix until it passes.

## Report

Fill the `worker-protocol` report. Under Notes, list any migration added.
```

The rest of that persona goes elsewhere: "you own services/api" is a line in `.apex/LIVE.md`, "you may push to
staging" is `- Allowed: push to the staging remote.` in `RULES.md`, the open lines of `TASKS.md` become ledger items,
and "you are Bob" and "remember" are dropped.

Frontmatter rules, checked by `apex skill check`:

| Field | Rule |
|---|---|
| `name` | equals the directory name; `^[a-z0-9][a-z0-9-]{0,63}$` |
| `description` | one line, at most 1024 characters; say what work it does and when to pick it, since search ranks on it |
| `metadata.kind` | `worker-skill` (others: `orchestrator`, `agent`, `protocol`, `skill`) |
| `metadata.tier` | `fast`, `standard`, `deep`, `audit` or `security`; never a model name |
| `metadata.tags` | comma-separated words the Orchestrator would search for; defaults to the domain folders |
| `metadata.inputs`, `outputs`, `verify` | what the brief must give, what comes back, how done is proved |
| `metadata.requires` | skills loaded with this one, such as `git-discipline` |

A persona that is not a kind of work but a tool the operator calls by hand (a formatter, a release checklist) can stay
a plain skill: the same file with `metadata.kind: skill`.

This shape is what lets one Orchestrator route across hundreds of skills without reading them. It learns in three
levels and stops at the first that answers:

```bash
apex skill search add endpoint --kind worker-skill --limit 10   # 1. find: one line per skill
apex skill show backend/api-endpoint                            # 2. decide: tier, inputs, outputs, verify, path
                                                                # 3. use: the worker loads the body, not the Orchestrator
apex skill check                                                # validate every skill in the repo
apex skill check backend/api-endpoint                           # or one
```

Group with domain folders (`backend/`, `infra/`, `docs/`) once names would collide or exceed a few dozen; a skill is
then addressed as `domain/name`. Delete each old persona and command file only after its lines are all placed.

### Step 4. Sort memory and rules into durable state

Sort one entry at a time, from harness memory, monolithic framing files and `.cursor/rules/*`. One entry per rule, not
per file.

| Entry | Goes to | Form |
|---|---|---|
| a fact every role needs (layout, build command, environments) | `.apex/LIVE.md`, shared and tracked; workers read it | short entries, rewritten when stale |
| a fact only the Orchestrator needs (what is pending outside the ledger, machine quirks) | `.apex/orchestrator/LIVE.md`, private | the same |
| an operator decision that grants something | `.apex/orchestrator/RULES.md` | `- Allowed: <action, target, condition>` |
| an operator decision phrased as a ban ("never push to origin") | `RULES.md` as the grant it implies (`- Allowed: push to fork.`), plus a `Don't:` in a reminder skill if the ban guards a real risk | |
| a trap, a limit, a convention, a how-to | `.apex/orchestrator/skills/<topic>.md` | lines starting `Do:`, `Don't:`, `Trap:`, `Good to know:` |
| procedure (session steps, "update the task list") or stale state (what was running) | dropped | |

`RULES.md` holds grants only: zero dates, zero history, zero bans. Each rule covers a kind of work, not one item, and
only the operator decides rules, so show the operator the file before relying on it. Whatever no rule allows still
needs the operator's approval. [Part 4](#part-4-legacy-reference-per-agent-private-folders-claude-layout) has the
full form with examples.

```markdown
# Rules

- Allowed: commit after a ledger item closes, without asking.
- Allowed: merge a worker back after its Verify command passes.
```

```markdown
# Database

- Do: run migrations one at a time.
- Don't: run the integration suite against the shared staging database.
- Trap: the seed script silently truncates `audit_log`.
- Good to know: local Postgres listens on 5433, not 5432.
```

Deleting migrated harness memory touches user data outside the repo (for Claude Code,
`~/.claude/projects/<slug>/memory/`): do it only on the operator's explicit yes. For Claude Code, also turn
auto-memory off (step 6) so the two stores cannot drift.

### Step 5. Migrate open work items

The ledger is written only through `apex` verbs. Never hand-edit an item's frontmatter, and never copy a foreign item
file into the ledger: ids are allocated by `apex add`. Always pass `--agent orchestrator` explicitly; there is no
default role.

```bash
apex add task "Rate-limit the public search endpoint" --agent orchestrator   # prints the new item's path
apex add followup "Remove the legacy search flag" --agent orchestrator       # found during another item's work
apex start 001 --agent orchestrator                                          # the item being worked now
apex close 002 "done: flag removed in 4f2a9c1" --agent orchestrator          # history you choose to carry
apex list --status all --agent orchestrator
```

Per old item, in the old order:

1. `apex add <task|followup> "<title>" --agent orchestrator`. The title is the old heading without its old id.
2. Write the body **below** the frontmatter:

   ```markdown
   ## Goal
   From: user
   <the old task text and its done-when, checkable>

   ## Approach

   ## Log
   migrated from TASKS.md line 14; old status: in progress
   <old progress notes>

   ## Outcome
   ```

3. Set the status: `apex start <id>` for work in progress, `apex close <id> "<reason>"` for finished history
   (`done: <result>` or `dropped: <why>`); an open item needs no call.

Fix a wrong title with `apex retitle <id> "<title>" --agent orchestrator` and a wrong reason with `apex reopen` then
`apex close`; never file the item again. Closed foreign history may also simply stay where it is: only open work must
move. Carry the newest foreign handoff's state, next step and open questions into
`.apex/ledger/orchestrator/handoff.md` (format in `state-protocol`), and drop superseded ones.

### Step 6. Shared config and line endings

| File | Content |
|---|---|
| `.apex/LIVE.md` | `templates/shared-LIVE.md`, plus the shared facts from step 4 |
| `.apex/git.md` | `templates/git.md`: commit, push and worker-merge permissions, message format, branch naming, remotes, PR template |
| `.apex/tiers.md` | `templates/tiers.md`: maps `fast`, `standard`, `deep`, `audit` and `security` to your harness's models or providers; keep only the harness columns you use |
| `AGENTS.md` | `# <repo>` and the section from `templates/AGENTS-section.md`; keep your own project description, move procedure out of it into skills |
| `.gitattributes` | the line `/.apex/** text eol=lf`, so CRLF cannot corrupt ledgers on Windows |

```bash
grep -qxF '/.apex/** text eol=lf' .gitattributes 2>/dev/null || printf '/.apex/** text eol=lf\n' >> .gitattributes
git config core.autocrlf false       # on Windows, in every clone and worktree
```

In `tiers.md`, a cell left `default` runs on the harness's default model, and `none` means no model fits: a role that
needs that tier stops and says so. `security` has no fallback, so map it or accept that security lenses stop.

Harness adapters, only for the harnesses the repo uses:

- **Claude Code:** `CLAUDE.md` is one line, `@AGENTS.md` (plus the adapter note from `templates/CLAUDE-section.md`),
  and `.claude/settings.json` holds `"autoMemoryEnabled": false`, merged into the existing JSON.
- **Gemini / Antigravity:** `GEMINI.md` is one line, `@AGENTS.md`.
- **Codex, T3 Code, generic `.agents`:** `AGENTS.md` is read as is.

Then verify with [Part 5](#part-5-post-migration-verification-and-troubleshooting), commit per `git-discipline`
(explicit paths; the private folder and `.apex/workers/` never appear), and start the first session with
`/orchestrator`. When the harness does not list it, tell the session to read `.agents/skills/orchestrator/SKILL.md`
and follow it.

## Part 2: Paned Multi-Agent Collaboration to Orchestration

For a repo that runs legacy peer Agents under `.claude/`, typically three panes such as `main`, `web-dev` and
`research`, each with `.claude/skills/<a>/SKILL.md`, `.claude/ledger/<a>/` and `.claude/apex/<a>/`.

| Option | Result | Downtime |
|---|---|---|
| **A. Phased Coexistence** | the Orchestrator runs in `.apex/` beside Agents that stay in `.claude/` | none |
| **B. Full Consolidation** | one Master Orchestrator; the Agents become worker skills | one quiet point per Agent |
| **C. Layout Move Only** | the same peer Agents, living in `.apex/` + `.agents/skills/` | one quiet point for the repo |

A leads naturally into B: coexist first, then fold in one Agent at a time. Nothing is required of a repo that wants
none of this: `apex` reads either layout.

`apex` resolves the layout **per Agent**, by where its role card is (`internal/layout`): a card at
`.agents/skills/<a>/SKILL.md` puts that Agent on the `apex` layout, a card at `.claude/skills/<a>/SKILL.md` on the
legacy one. `apex where --agent <a>` prints which.

> **Split-ledger guard.** If an Agent's role card is on one layout and a ledger for the same name is on the other,
> every `apex` verb for that Agent exits `4` with `split ledger: <a> has its skill at ... but a ledger at ...`,
> because reading one half would silently lose the other. So never copy or move an Agent's `SKILL.md` into
> `.agents/skills/` while its `.claude/ledger/<a>/` still exists. A role card and its ledger always move in the same
> step, and a new worker skill is never given the name of an Agent whose legacy ledger is still in place.

### Option A: Phased Coexistence (zero downtime)

The Agents are not touched.

1. Install both workflows, so `agent-protocol` stays available to the Agents:
   `scripts/install.sh --workflow both --harness <preset>`.
2. Stand up the Orchestrator exactly as in [Part 1](#step-2-establish-the-master-orchestrator), steps 2 and 6, or ask
   `/apex-init` to add an Orchestrator beside the Agents. The name `orchestrator` must not be an existing Agent's.
3. Shared files now exist twice, because each side reads its own state directory:

   | File | Agents read | Orchestrator reads | During coexistence |
   |---|---|---|---|
   | shared notes | `.claude/LIVE.md` | `.apex/LIVE.md` | keep the facts in `.claude/LIVE.md`; put one line in `.apex/LIVE.md` saying so |
   | git conventions | `.claude/git.md` | `.apex/git.md` | copy it, and add the `Worker merge` row |
   | tier map | none | `.apex/tiers.md` | new |

4. Framing: write `AGENTS.md`, and add the line `@AGENTS.md` at the top of `CLAUDE.md`. Leave the `## Apex Agents`
   section of `CLAUDE.md` as it is while any Agent still starts from it.
5. Work crosses over through the ledgers. The Orchestrator is not a peer: the operator reaches it, and it files work
   for an Agent the way a peer would, `apex add task "<title>" --agent <a>`. The reverse works the same way, with
   `--agent orchestrator`.
6. Check: `apex where --agent orchestrator` prints `layout: apex`, `apex where --agent <a>` prints `layout: legacy`,
   and `apex list --status all --agent <a>` gives the same counts as before.

When an Agent reaches a quiet point, fold it in with Option B, or move it with Option C.

### Option B: Full Consolidation into one Master Orchestrator

Per peer Agent `<a>`, with the Orchestrator already established (Part 1, steps 2 and 6):

1. **Finish or re-file its open items.** Let the Agent finish its in-progress item, or stop it and log where it
   stopped. Then, for each item still open:

   ```bash
   apex list --status in-progress,open --agent <a>
   apex add task "<title>" --agent orchestrator          # prints .apex/ledger/orchestrator/NNN.md
   # body: "## Goal" starts with "From: <a>/007", then the old Goal, Approach and Log
   apex close 007 "moved to .apex/ledger/orchestrator/NNN" --agent <a>
   ```

   The `From: <old-agent>/NNN` line and the close reason point at each other, so the history can be followed both
   ways.
2. **Carry the handoff and notes.** Copy the State, Next and Open threads that still matter from
   `.claude/ledger/<a>/handoff.md` into the Orchestrator's handoff. Sort `.claude/apex/<a>/LIVE.md`, `RULES.md` and
   `skills/*.md` into `.apex/orchestrator/` by the table in [Part 1, step 4](#step-4-sort-memory-and-rules-into-durable-state).
   A grant given to one Agent is not automatically the Orchestrator's: show the operator the merged `RULES.md`.
3. **Archive the closed ledger and retire the role card, in one commit:**

   ```bash
   mkdir -p .apex/ledger/archive
   git mv .claude/ledger/<a> .apex/ledger/archive/<a>     # history, read-only from now on
   git rm -r -q .claude/skills/<a>                        # after step 4 has taken what it needs
   rm -rf .claude/apex/<a>                                # untracked; only after step 2
   ```

   Doing both together means no state is left that could trip the split-ledger guard.
4. **Convert the role prompt into stateless worker skills**, as in
   [Part 1, step 3](#step-3-decompose-personas-and-commands-into-stateless-worker-skills). The role card's ownership
   lines become `tags` and Scope hints, its working method becomes skill steps, and its peers, voice and session
   procedure are dropped. Name skills after the work (`web/component`, `research/literature-scan`), not after the
   Agent.

Once, for the repo:

5. **Merge shared facts:** `git mv .claude/LIVE.md .apex/LIVE.md` and `git mv .claude/git.md .apex/git.md` (merge by
   hand if Option A already created them), then delete entries that only described the peers.
6. **Framing:** replace the `## Apex Agents` peer instructions in `CLAUDE.md` with `AGENTS.md` (from
   `templates/AGENTS-section.md`) and a `CLAUDE.md` that holds `@AGENTS.md`.
7. **Install:** `scripts/install.sh --workflow orchestration --harness <preset>` sweeps the now unused
   `agent-protocol` skill.
8. Verify with Part 5 and commit.

### Option C: Layout move only (keep peer Agents, leave `.claude/`)

Keeps Multi-Agent Collaboration and moves it to the harness-neutral layout. `/apex-init` does this on request ("move
the legacy layout"). By hand, with every Agent at a quiet point and a clean tree:

```bash
mkdir -p .apex .agents/skills
git mv .claude/LIVE.md .apex/LIVE.md
git mv .claude/git.md  .apex/git.md
git mv .claude/ledger  .apex/ledger                 # every .claude/ledger/<a>/ at once
for a in <a> <b> <c>; do
  git mv ".claude/skills/$a" ".agents/skills/$a"    # role card, in the same step as its ledger
  [ -d ".claude/apex/$a" ] && mv ".claude/apex/$a" ".apex/$a"   # untracked private folder; keeps its .gitignore
done
```

Then:

1. Other skills under `.claude/skills/` that are not Apex role cards may stay, or move to `.agents/skills/` with
   `git mv`.
2. Add `.apex/tiers.md` and the `.gitattributes` line ([Part 1, step 6](#step-6-shared-config-and-line-endings)).
3. Replace the `## Apex Agents` section of `CLAUDE.md` with `AGENTS.md` plus `@AGENTS.md`.
4. In each role card, change the notes row to `.apex/<a>/`, with the operator's approval, and rewrite in-repo
   references to the old paths (`git grep -nIF -e '.claude/ledger' -e '.claude/apex' -e '.claude/LIVE.md'`).
5. Check: `apex where` prints `layout: apex`; `apex list --status all --agent <a>` gives the same counts as before the
   move for every Agent; `apex skill check` exits 0.
6. Commit, then refresh each handoff's `checkout`, `branch` and `head`, since the move made every handoff stale:

   ```bash
   git rev-parse --show-toplevel      # checkout
   git rev-parse --abbrev-ref HEAD    # branch
   git rev-parse HEAD                 # head, the full SHA
   ```

   Write the three values into the frontmatter of each `.apex/ledger/<a>/handoff.md`. They ride in a later commit.

An Agent name of `ledger`, `workers` or `worktrees` is reserved on the `apex` layout: rename that Agent before moving
it. If your harness does not list skills from `.agents/skills/`, start an Agent by telling the session to read
`.agents/skills/<a>/SKILL.md` and follow it.

## Part 3: Machine installs, T3 Code and multi-harness setups

### Replacing a legacy install

| Found on the machine | What it is | Do |
|---|---|---|
| `~/.claude/bin/apex` or `apex.exe` | the v0.3 binary; it shadows the new `apex` on `PATH`, and the installer stops with exit `1` until it is gone | `example/scripts/uninstall.sh` (or `uninstall.ps1`), then `command -v apex` |
| Apex skills under `~/.claude/skills/`, no manifest | an install from the old `make install`, before the manifest existed; the installer reads it as "Multi-Agent Collaboration on `claude`" | re-run the installer with explicit `--workflow` and `--harness` |
| `${XDG_CONFIG_HOME:-~/.config}/apex/manifest` | a current install | re-run the installer to switch |

```sh
scripts/install.sh --check                                           # preflight only, writes nothing
scripts/install.sh --workflow orchestration --harness agents         # the default pair
scripts/install.sh --workflow both --harness claude,codex            # several harnesses at once
scripts/install.sh --harness custom --skills-dir /path/to/skills     # a directory you name
scripts/install.sh --custom                                          # asked for each choice
pwsh scripts/install.ps1 -Workflow orchestration -Harness t3         # Windows: same flags, PowerShell spelling
```

| Flag | Values |
|---|---|
| `--workflow` | `orchestration` (default), `multi-agent`, `both` |
| `--harness` | `agents` (default), `claude`, `codex`, `gemini`, `t3`, `custom`; comma list allowed |
| `--custom`, `--interactive` | ask for each choice; `--yes` never asks |
| `--prefix` | where `apex` goes, default `~/.local/bin`; must be on `PATH` |

`make install` still works and forwards to `scripts/install.sh` (`make install WORKFLOW=orchestration HARNESS=t3`).
Without flags, an existing install keeps its previous workflow and harness, so a machine upgraded from the old
`make install` stays on Multi-Agent Collaboration until you name the workflow.

**Manifest-based switching.** The installer records the workflow, the binary path and every skills directory in the
manifest. On the next run it sweeps both the previous directories and the new ones, removing every skill folder that
carries its `.apex-installed` mark, then installs the chosen set. So switching workflow or harness leaves nothing
stale, and a skill folder without the mark is yours: it is never overwritten or removed. If one of your folders has
the name of an Apex skill, the installer stops with exit `1` and asks you to move it first. A binary left at an old
`--prefix` is removed. `scripts/uninstall.sh` removes only what the manifest and the marks name.

`apex` must be on the `PATH` of the shell your harness runs commands in: every skill calls a bare `apex`.

### Adopting Apex on T3 Code

T3 Code routes work to several providers and creates child worktrees itself, so two things differ from the default.

1. **Install the multi-root preset.** `--harness t3` installs the skills into the `agents`, `claude`, `codex` and
   `gemini` user paths in one run, so whichever provider T3 Code picks for a thread finds them.
2. **Map tiers to T3 provider and model ids.** Call `orchestrator_capabilities` once to list the providers and models
   the T3 Code install offers, and write them into a T3 Code column of `.apex/tiers.md`:

   ```markdown
   | Tier | For | T3 Code |
   |---|---|---|
   | `fast` | search, lookups, mechanical edits | `<provider>` / `<small model id>` |
   | `standard` | most implementation and review work | `<provider>` / `<default model id>` |
   | `deep` | design, hard debugging, judging | `<provider>` / `<strongest model id>` |
   | `audit` | an independent second check | `<a different provider>` / `<model id>` |
   | `security` | security review | `<provider>` / `<model id>`, or `none` |
   ```

   Mapping `audit` to a different provider than `deep` is what makes the second opinion independent.
3. **Delegate with harness isolation.** The Orchestrator starts each worker through T3 Code's `delegate_task` with
   `isolation: "worktree"` and the provider and model for the skill's tier. The brief is unchanged: Goal, Inputs,
   Scope, Verify, Sync, plus `worker-protocol` and each assigned skill's path from `apex skill show`.
4. **Adopt, merge, prune.** When the child reports, register its worktree or branch, then use the normal worker
   verbs:

   ```bash
   apex worker adopt <worktree-or-branch> --slug <slug> --base <branch> --skill <a,b> --item orchestrator/<id>
   apex worker list                         # the adopted worker, with ahead, behind and dirty
   apex worker merge <id> --message "<type>(<scope>): <summary>"    # --no-ff into <base>; exit 5 on conflict
   apex worker prune                        # clears merged workers
   ```

   `adopt` writes `.apex/workers/<id>.json` for a worktree `apex` did not create. `apex worker remove <id>` only
   untracks an adopted worker; `apex worker remove <id> --purge` also deletes the external worktree and its branch.
   On a conflict, send a sync step to the same child worktree and retry the merge; never resolve in the base
   checkout.

Other harnesses keep the default: `apex worker spawn` creates the worktree, and the harness's own isolation is not
used. A team on several harnesses needs nothing more than a comma list at install (`--harness claude,codex,gemini`),
one column per harness in `.apex/tiers.md`, and the framing adapters of
[Part 1, step 6](#step-6-shared-config-and-line-endings).

## Part 4: Legacy reference: per-Agent private folders (`.claude/` layout)

For legacy repos that are not yet moving to `.apex/`. It moves each Agent's notes out of its tracked role card folder
into a private folder in the main checkout, and splits them into live notes, rules and reminder skills. On the `apex`
layout the same three files live in `.apex/<agent>/`.

### Before and after

```
before                                    after
<repo>/.claude/                           <repo>/.claude/
  skills/<agent>/SKILL.md                   skills/<agent>/SKILL.md      role card, tracked, unchanged role
  skills/<agent>/LIVE.md                    apex/<agent>/.gitignore      one line: *
                                            apex/<agent>/LIVE.md         live notes
                                            apex/<agent>/RULES.md        operator rules
                                            apex/<agent>/skills/*.md     reminder skills
  LIVE.md                                   LIVE.md                      shared notes, tracked, unchanged
  ledger/<agent>/                           ledger/<agent>/              tracked, unchanged
  git.md                                    git.md                       unchanged
```

`.claude/apex/<agent>/` ignores itself: its `.gitignore` holds `*`, which also ignores the `.gitignore`. Nothing in it
is ever staged, and the repo's root `.gitignore` is not touched. It lives only in the main checkout; an Agent in a
worktree reads it through `<repo>`, as it already does for the ledger.

### What goes where

| File | Holds | Written by | Form |
|---|---|---|---|
| `LIVE.md` | facts about the current state that outlive one item: where things are, what is installed, what is pending outside the ledger | the Agent | short entries, rewritten when stale |
| `RULES.md` | what the operator has decided the Agent may do | the Agent, only from the operator's words | approval-first, see below |
| `skills/<topic>.md` | reminders on how to do the work: traps, good-to-knows, do's and don'ts | the Agent | one topic per file |

A reminder skill is not an Agent: it never changes the role, the owned paths or the protocol. These files are plain
markdown that the Agent reads with a file read. They are not harness skills and are never discovered or invoked as
`/<name>`.

#### RULES.md

Rules are decided by the operator. Each one:

- grants something: it starts with `Allowed:` and names what the Agent may do;
- is specific: the action, the target and the condition, so it can be checked against one action;
- covers a kind of work ("any modelling pass"), not one ledger item, so it outlives the item that prompted it;
- carries no date, timestamp or history;
- contains no prohibition and no trap. A trap, a "don't" or a limit the operator sets (one heavy process at a time, a
  timeout on every tool call) goes in a reminder skill as a `Don't:` or `Do:` line. Whatever no rule allows still
  needs the operator's approval.

```markdown
# Rules

- Allowed: commit after a ledger item closes, without asking.
- Allowed: push `fresh-slate` and `master` to `fork` when the operator says "push".
- Allowed: delete a branch on `fork` after it is merged to `master`.
```

Rewrite a rule in place when the operator changes it; delete it when the operator withdraws it.

#### Reminder skills

```markdown
# Install

- Do: run the installer after changing anything under `skills/`; nothing is live until then.
- Don't: search git-ignored files in adopted repos.
- Trap: a legacy `~/.claude/bin/apex` shadows the new binary on `PATH`.
- Good to know: `pwsh` is not installed on this machine; PowerShell edits are read-checked only.
```

### Steps, per Agent

Run in the main checkout. `<a>` is the Agent's directory name under `.claude/skills/`.

1. Create the folder:

   ```bash
   mkdir -p .claude/apex/<a>/skills && printf '*\n' > .claude/apex/<a>/.gitignore
   ```

2. Sort every entry of `.claude/skills/<a>/LIVE.md`, one at a time:

   | Entry | Goes to |
   |---|---|
   | an operator decision that grants something | `RULES.md`, rewritten as `Allowed: ...` |
   | an operator decision phrased as a ban ("never push to origin") | `RULES.md` as the grant it implies ("Allowed: push to `fork`"), plus a `Don't` in a reminder skill if the ban carries a real risk |
   | a trap, a convention, a do or don't, a how-to | `skills/<topic>.md` |
   | a fact about the current state | `LIVE.md` |
   | stale or already covered by the ledger | nothing |

   Drop dates and "since"/"as of" wording from rules as they move. Show the operator the new `RULES.md` before step 3:
   only the operator decides rules.

3. Remove the old file from the tree and from the index:

   ```bash
   git rm -q .claude/skills/<a>/LIVE.md
   ```

4. In the role card, point the live notes row at the new folder:

   | Owns | Path |
   |---|---|
   | Live notes, rules, reminder skills | `.claude/apex/<a>/` |

   The role card is the operator's file: make this one edit with the operator's approval and change nothing else.

5. Check:

   ```bash
   git check-ignore -q .claude/apex/<a>/LIVE.md && echo ignored
   git status --porcelain -- .claude/apex          # prints nothing
   test ! -e .claude/skills/<a>/LIVE.md && echo moved
   grep -n '^- ' .claude/apex/<a>/RULES.md | grep -v '^[0-9]*:- Allowed: '   # prints nothing
   ```

6. Commit the removal and the role card edit per `git-discipline`. The new folder never appears in the commit.

`.claude/LIVE.md`, the ledger, the handoff and `.claude/git.md` do not move. The current `agent-protocol`, `sign-off`
and `git-discipline` skills already read the private folder and never stage it, so install the current skills before
a migrated Agent starts.

## Part 5: Post-migration verification and troubleshooting

### Checklist

Run in the main checkout. Every command exits 0.

```bash
apex where --agent orchestrator                        # layout: apex; ledger, handoff and private under .apex/
apex list --status all --agent orchestrator            # every migrated item, with the planned statuses
apex skill check                                       # every repo skill has valid frontmatter
apex skill list --kind worker-skill                    # the skills you decomposed
apex worker list                                       # no stale workers
git check-ignore -q .apex/orchestrator/LIVE.md && echo private-ignored
git check-ignore -q .apex/workers/x.json && echo workers-ignored
git check-attr eol -- .apex/LIVE.md                    # eol: lf
git status --porcelain                                 # only the files this migration wrote or moved
```

Then, by eye:

- For each Agent kept (Options A and C): `apex where --agent <a>` and `apex list --status all --agent <a>` show the
  layout you expect and the same item counts as before.
- `AGENTS.md` holds the `## Apex Agents` section with its `Workers` line; `CLAUDE.md` or `GEMINI.md`, if used, holds
  `@AGENTS.md`.
- `git status --porcelain -- .apex/orchestrator .apex/workers` prints nothing.
- `command -v apex` prints the path the installer reported, not a path under `~/.claude/bin`.
- End to end: start `/orchestrator`, file one small item, and let it run a worker through spawn (or adopt), verify,
  merge and prune.

### Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| exit `4`, `split ledger: <a> has its skill at ... but a ledger at ...` | the role card is in `.agents/skills/<a>/` while the ledger is still in `.claude/ledger/<a>/` (or the reverse) | put the two on one layout: `git mv .claude/ledger/<a> .apex/ledger/<a>`, or move the card back to `.claude/skills/<a>/`. If `<a>` is a new worker skill that merely reuses a retired Agent's name, archive the old ledger (Part 2, Option B, step 3) or rename the skill |
| exit `4`, `CRLF line endings` on a ledger item or handoff | on Windows, `core.autocrlf=true` rewrote `.apex/` files to CRLF at checkout | `git config core.autocrlf false`; add `/.apex/** text eol=lf` to `.gitattributes`; then normalize: `git add --renormalize .apex && git commit`, and `git rm -r -q --cached .apex && git reset -q --hard` on a clean tree to rewrite the working files as LF |
| exit `4` on one item, any other message | frontmatter was edited by hand or an item file was copied in | never repair frontmatter by hand; restore the file from git and redo the change with `apex` verbs |
| the installer stops: a legacy Apex v0.3 install shadows the new `apex`; or `apex where` is an unknown verb | `~/.claude/bin/apex` is earlier on `PATH` | `example/scripts/uninstall.sh` (or `.ps1`), restart the shell, check `command -v apex` |
| exit `1`, `no agent "orchestrator"` | `.agents/skills/orchestrator/SKILL.md` is missing, or the command ran in a different repo | Part 1, step 2 |
| exit `64`, `--agent is required` | a ledger verb ran without `--agent` | pass `--agent orchestrator`; there is no default role |
| exit `64`, `agent name ... is reserved on the apex layout` | an Agent named `ledger`, `workers` or `worktrees` was moved to `.agents/skills/` | rename the Agent before the move |
| exit `3` on `apex start` | another item is in progress and the ledger has no `concurrent` marker | create the empty file `.apex/ledger/orchestrator/concurrent` |
| a worker edits the ledger, a handoff or `LIVE.md`, or runs `apex add` / `close` | its session never saw the `Workers` rule: `AGENTS.md` lacks the line, or the delegate prompt did not name `worker-protocol` | restore the `## Apex Agents` section from `templates/AGENTS-section.md`; name `worker-protocol` in every brief; before merging, `git -C <wt> diff --stat <base>...HEAD -- .apex` must print nothing, else have the worker revert those commits. State is written by the Orchestrator only |
| a T3 Code child branch does not show in `apex worker list` | the harness created the worktree, so `apex` has no manifest for it | `apex worker adopt <worktree-or-branch>` |
| `apex worker merge` exits `5` | the worker branch conflicts with `<base>`; the merge was aborted cleanly | sync inside the worker's worktree (merge or rebase `<base>`, rerun Verify, commit), then retry; never resolve in the base checkout |
| the harness does not list `/orchestrator` | it does not load `.agents/skills/`, or was not restarted after the install | restart it; else tell the session to read `.agents/skills/orchestrator/SKILL.md` and follow it |
| the installer stops: `<dir>/<skill> exists and was not installed by apex` | a folder of yours has an Apex skill's name | move or rename your folder; the installer never overwrites an unmarked skill |
| old notes reappear, or facts contradict `.apex/` | harness auto-memory is still on | for Claude Code set `"autoMemoryEnabled": false` in `.claude/settings.json`; turn off any memory another harness keeps |
