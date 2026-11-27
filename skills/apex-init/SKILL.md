---
name: apex-init
description: Adopt Apex in a repo. Inspects the repo and does exactly one thing. It sets up the Orchestration-based Workflow from a blank slate (the default), sets up Multi-Agent Collaboration on request, migrates a foreign layout of role cards, ledgers, handoffs and memory, moves a legacy layout to the apex layout, or reports the repo conformant and changes nothing. Use once per repo, on /apex-init or when the operator asks to set up or migrate Apex.
metadata:
  kind: skill
---

# apex-init

A single-use session. Inspect, route to exactly one outcome, finish. `<root>` is the main checkout; `apex where`
prints it, and `dirname "$(git rev-parse --path-format=absolute --git-common-dir)"` gives it before any Apex file
exists.

Templates are in this skill's `templates/` directory.

| Placeholder | Value | Example |
|---|---|---|
| `{{agent}}` | role name: directory, ledger and `--agent` value | `web-dev` |
| `{{Agent}}` | display name: `{{agent}}` with each `-`-separated word capitalized, hyphens kept | `Web-Dev` |
| `{{repo}}` | `<root>`'s directory name | `shop` |
| `{{one-line role}}` | one sentence ending in a period | `Builds and deploys the web apps.` |
| `{{role}}` | the `## Role` body: the one-line role, or the old card's role content | |
| `{{...}}` in `git.md` | the row's value; the text inside is a hint | |

A `description` in a template is a double-quoted string: escape any `\` in the role as `\\` and any `"` as `\"`.

## 1. Inspect

Collect these facts before routing. Read only; change nothing yet.

| Fact | How |
|---|---|
| Layout | `apex where --json`: `layout` is `apex` or `legacy` |
| Orchestrator | `<root>/.agents/skills/orchestrator/SKILL.md` with `metadata.kind: orchestrator` |
| Apex Agents | each role skill with `metadata.kind: agent`, or one that loads `agent-protocol`, under `apex where`'s `skills` |
| Unmigrated notes | an Apex Agent with a tracked `LIVE.md` next to its skill, or without a private folder `.gitignore` |
| Foreign ledgers | directories of numbered item files kept outside `<state>/ledger/`, such as `DevLedger/` with `INDEX.md` and `NNN-slug.md` |
| Foreign handoffs | handoffs of a role defined in this repo kept outside `<state>/ledger/<a>/handoff.md`, such as `docs/handoff/*.md`. Notes files do not count |
| Foreign role cards | skills that describe an agent role but are not Apex roles, such as a `dev-agent/` skill |
| Other-home files | handoffs, ledgers or cards of an Agent whose home is another repo. Reported, never migrated, and not counted as foreign layout |
| Harness | which harnesses the repo uses: Claude Code (`CLAUDE.md`, `.claude/`), Codex (`~/.codex`), T3 Code (the `orchestrator_capabilities` tool, or `~/.t3`) and Gemini (`~/.gemini`); ask the operator when nothing shows it |

### Claude Code facts

| Fact | How |
|---|---|
| Claude Code in use | a `<root>/CLAUDE.md` or `<root>/.claude/` exists, or the operator says so |
| Legacy role cards | `<root>/.claude/skills/<x>/SKILL.md` (legacy): an Apex Agent if it loads `agent-protocol`, else a foreign role card if it describes an agent role |
| Claude Code project memory | `find ~/.claude/projects -maxdepth 3 -path "*/<slug>*/memory/*.md"` (glob-safe in every shell), where `<slug>` is `<root>` with every non-alphanumeric character replaced by `-` |

The Claude Code memory slug maps `/` and `-` alike, so a `<slug>-*` match counts only when a directory inside
`<root>` produces it and no sibling directory of `<root>` does (`ls -d <root>-*`); when both could, ask the operator.

## 2. Route

| State found | Outcome |
|---|---|
| `apex list --status all --agent <a>` exits non-zero for any role | Stop. Report the role, exit code and message; change nothing |
| No Apex roles, and no foreign role cards, ledgers, handoffs or project memory | 3. Orchestration blank slate, or 4. Collaboration blank slate when the operator asks for named Agents |
| Any foreign role card, ledger or handoff, or project memory with no Apex roles | 5. Migrate |
| `layout: legacy` and the operator asks to move it | 6. Legacy layout move |
| Apex roles present, no foreign layout, and only unmigrated notes or the shared signals in 8 fail | 9. Private folders for roles with unmigrated notes, 7. Shared setup for the failed signals, then 10 |
| Apex roles present and every signal in 8 holds | 8. Conformant |

A legacy layout that is otherwise conformant is reported conformant, with the move offered in one line. When the route
is unclear, show the operator the facts from step 1 and ask.

Before the first write on any route, record the output of `git -C <root> status --porcelain`: the files already
modified hold the operator's own edits, and section 10 needs the list.

## 3. Blank slate: Orchestration-based Workflow

The default.

1. Pick the starter worker skills: `implement`, `review`, `research`, all three unless the operator names fewer. A
   starter whose name an existing skill already uses is skipped and reported.
2. Write:

   | Path | From |
   |---|---|
   | `<root>/.agents/skills/orchestrator/SKILL.md` | `templates/orchestrator-SKILL.md`; `{{role}}` is the operator's words, else `Runs this repo's work through stateless workers.` |
   | `<root>/.apex/orchestrator/.gitignore` | one line, `*` |
   | `<root>/.apex/orchestrator/LIVE.md` | `templates/LIVE.md`, `{{Agent}}` as `Orchestrator` |
   | `<root>/.apex/orchestrator/skills/` | empty directory |
   | `<root>/.apex/ledger/orchestrator/concurrent` | empty file |
   | `<root>/.apex/workers/.gitignore` | one line, `*` |
   | `<root>/.agents/skills/<starter>/SKILL.md` | `templates/starters/<starter>/SKILL.md`, as is |

3. Do the shared setup in 7.
4. Check: `apex list --status all --agent orchestrator` exits 0 with no items; `apex skill check` exits 0;
   `apex worker list` exits 0 with no workers.
5. Commit as in 10.
6. Report the files written and how to start: `/orchestrator`.

To add a worker skill later, write `.agents/skills/<name>/SKILL.md` from `templates/worker-SKILL.md` and run
`apex skill check <name>`.

## 4. Blank slate: Multi-Agent Collaboration (legacy workflow)

Only when the operator asks for named peer Agents.

1. Take the Agents from the operator's request when it names them; otherwise ask. Per Agent: a name matching
   `^[a-z0-9][a-z0-9-]*$` that no existing skill uses, and a one-line role. Ask for more role text only if the
   operator offers it.
2. Per Agent:
   - write `<root>/.agents/skills/<agent>/SKILL.md` from `templates/agent-SKILL.md`;
   - create the private folder: `mkdir -p <root>/.apex/<agent>/skills && printf '*\n' > <root>/.apex/<agent>/.gitignore`;
   - write `<root>/.apex/<agent>/LIVE.md` from `templates/LIVE.md`.
3. Do the shared setup in 7.
4. Check with `apex list --status all --agent <agent>` per Agent (exit 0, no items) and `apex skill check`.
5. Commit as in 10.
6. Report the files written and how to start: `/<agent>`.

A legacy layout is written only when the operator asks for it in words: then every path above moves under `.claude/`
(legacy: `.claude/skills/<agent>/`, `.claude/apex/<agent>/`), and section 7 writes the legacy files.

## 5. Migrate

Migration moves and rewrites existing state into the `apex` layout. Nothing is moved or deleted before the operator
approves the plan, and old files are removed only after the operator confirms the migrated result. Each migrated
role becomes a Collaboration Agent, unless the operator asks to fold the work into an Orchestrator's ledger.

### 5.1 Read

Read every foreign ledger item, index, handoff and role card, and every project memory file. Collect every free-text
shape used to cite an old item, such as `DevLedger 033`, `(025)`, `R007`, `M011`, `Master-Tasks/M011-...`,
`XageLedger 018`.

### 5.2 Plan

Show the operator one plan and wait for approval.

| Plan section | Contents |
|---|---|
| Roles | old role card or ledger to new role name; default is the old name without an `-agent` suffix. With nothing to map from (memory files only), ask as in 4 step 1 |
| Shared ledgers | a ledger several roles work, such as `Master-Tasks/` kept by research: folds into its keeper's ledger after the keeper's own items; its per-role columns become peer references |
| Items | per old item: old path, new id, title, kind, new status, close reason |
| Ids | one map per role: its old items renumbered from `001` in old order, shared-ledger items last |
| Reference shapes | every shape from 5.1, each with its rewrite; the operator confirms the list |
| Status conflicts | every item whose file status differs from its `INDEX.md` row or from its old handoff; the item file wins, the operator confirms, and a handoff's differing claim goes into the new handoff's State as "check before resuming" |
| In progress | at most one item per role; the operator picks it when several old items are running |
| Handoffs | old path to `.apex/ledger/<role>/handoff.md`, or dropped, and why |
| Memory | per rule, by the sort table in 9: the role's `LIVE.md`, `RULES.md` or a reminder skill, `.apex/LIVE.md`, or dropped |
| References | in-repo files citing old items or paths (to rewrite), and references in other repos (listed only) |
| Left alone | other-home files and anything else that stays, and why |
| To delete after confirmation | every old file the migration replaces |

**Title** is the item's H1, else the `INDEX.md` task column. Drop the item's own old id from it, such as `M003` in
`M003 run5 setup`, and rewrite any other old id to its peer form. **Created** stays the migration date that `apex add`
writes. **Kind** is `task` unless the old item says it was found during another item's work, then `followup`.

| Old status says | New status | Reason |
|---|---|---|
| done | `closed` | `done: <short result>` |
| abandoned, handed to another role | `closed` | `dropped: <why>` |
| done, but waiting on someone | `closed`; the waiting part goes into the handoff's Open threads unless a later item or this migration resolved it (list those in the plan) | `done: <short result>` |
| in progress, running, launched | `in-progress` for the one picked, `open` for the rest | none |
| open, awaiting someone | `open` | none |

A reason is one line that does not repeat its prefix (`done: 5a passes all gates`, not `done: done`), with old
references rewritten by the id map. An item marked running or launched may be stale. Check it against `INDEX.md`, the
old handoff and `git log`, and offer to close the finished ones in the same prompt that asks for the in-progress pick.

**Reference forms.** An item's own-ledger reference is `NNN`. Every other reference, from a peer's item, code or
docs, is `<repo>:<role>/NNN`, where `<repo>` is the home repo's directory name, such as `xage-guard:xage/018`. Old
shapes citing a role not migrated here, such as `XageLedger 018`, stay as they are. A bare number may cite another
role's ledger: resolve each occurrence from its context, and list the uncertain ones in the plan. A `memory <name>`
mention becomes a reference to the notes file that took that rule. Where old text already pairs a shape with its
path, such as `M011 (Master-Tasks/M011-x.md)`, write the new reference once, not twice.

**Handoffs.** Of several dated handoffs for one role, the newest is carried and the superseded ones are dropped, not
merged. README and index files are dropped. A notes file becomes `LIVE.md` facts or stays where it is; ask which.

**Memory.** One notes entry per rule, not per file. Keep facts; drop procedure (protocol and session steps) and
stale state (what was running or last committed). A fact several roles need goes to `.apex/LIVE.md`, once. Index
files such as `MEMORY.md` are dropped.

**References.** Search the main checkout only: tracked and untracked text files, honoring ignore rules, skipping
worktrees and scratch directories. Never search ignored files: they hold model weights, run outputs and
virtualenvs, and reading them can exhaust memory on a machine that is training.

Pass each old shape and path as a fixed string, one `-e` each, so `(025)` and `.` match literally:

```bash
git -C <root> grep -nIF --untracked -e '<shape>' -e '<path>' -- . \
  ':(exclude).apex/workers' ':(exclude,glob)**/node_modules/**' \
  ':(exclude,glob)**/.venv/**' ':(exclude,glob)**/venv/**' ':(exclude,glob)tmp/**' ':(exclude,glob)**/scratch*/**'
```

In a Claude Code repo, also exclude `':(exclude).claude/worktrees'` and read the legacy `.claude/` protocol files
directly; the search may not reach them.

Search other repos too, read only, with the same command: the home repo of each other-home role found in step 1, and
any repo the operator names.

Anything the plan cannot map or merge, ask about.

### 5.3 Execute

Before the first write, the working tree must be clean, or the operator accepts carrying the uncommitted edits into the
migration. Then, in this order, because `apex` needs the role's `SKILL.md` to exist:

1. **Role cards.** Write `.agents/skills/<role>/SKILL.md` from `templates/agent-SKILL.md`. Put the old card's role
   content under `## Role`: ownership, peers, working method, voice. Drop what the protocol now supplies: session-start
   steps, ledger and handoff procedure, pointers to memory or an index.
2. **Notes.** Create each role's private folder and `LIVE.md` as in 4 step 2, and `.apex/LIVE.md` from
   `templates/shared-LIVE.md` if missing, then fold in the planned memory entries.
3. **Items.** Per role, in id map order:
   1. `apex add <kind> "<title>" --agent <role>`; it prints the new path. Stop and ask if its id differs from the map.
   2. Write the body below the frontmatter; never touch the frontmatter. Rewrite references by the id map.

      | Section | From the old item |
      |---|---|
      | `## Goal` | `From:` line (`user` for the operator, a reference form above for a peer, `<role>` for a peer that asked without an item), then the old task and done-when text |
      | `## Approach` | the old approach if stated, else leave empty |
      | `## Log` | first line `migrated from <old path>; old status: <text>; created <old date>` (drop the date if the old item has none), then the old progress notes, then each other old section (Rules, Gates, Owners, Design) as a `###` subsection |
      | `## Outcome` | the old result, for closed items only |

      For an item not closed, the old result or next-step text goes in the Log.
   3. Set status: `apex close <id> "<reason>" --agent <role>`, or `apex start <id> --agent <role>` for the one
      in-progress item. Open items need no call.

   Fix a wrong title with `apex retitle <id> "<title>" --agent <role>` and a wrong reason with `apex reopen` then
   `apex close`; never file the item again.
4. **Handoffs.** Write `.apex/ledger/<role>/handoff.md` in the format from `state-protocol`. Its frontmatter is filled
   after the commit, in 10. Carry State, Next and Open threads from the carried handoff, plus the waiting parts from
   the status table; rewrite references by the id map. Procedure text that belongs to the role card is dropped.
5. **References.** Rewrite every in-repo reference the plan lists: comments, docs, every notes file. Rewrite each file
   in one pass from the map; chained replacements turn old `014 -> 013` into `012`. A string literal that reaches
   program output is not rewritten; list it for the operator.
6. **Shared setup** in 7.

### 5.4 Confirm, then remove

1. Per role, `apex list --status all --agent <role>`: the item count equals the planned count, and statuses match the
   plan.
2. Rerun the reference search from 5.2: nothing in the repo still cites an old shape or path, except what the plan
   leaves alone, the files on the delete list, and the Log sections of migrated items, which keep history as written.
3. Show the operator the results of 1 and 2, the delete list, and the references in other repos for them to rewrite.
   On confirmation, delete the old ledgers, indexes, handoffs and role card directories. Otherwise fix what the
   operator names and ask again.
4. Deleting migrated Claude Code memory files touches `~/.claude` (Claude Code user data): ask for an explicit yes,
   and delete only on it. Without one, the deletion stays a pending step.
5. Commit as in 10.

## 6. Legacy layout move

Moves a legacy repo, everything under `.claude/` and framed by `CLAUDE.md`, to the `apex` layout. Only on the
operator's request, after a plan the operator approves. The working tree must be clean first.

| Legacy path | New path | How |
|---|---|---|
| `.claude/skills/<a>/` of each Apex role (legacy) | `.agents/skills/<a>/` | `git mv`; other legacy Claude Code skills stay |
| `.claude/ledger/` (legacy) | `.apex/ledger/` | `git mv` |
| `.claude/apex/<a>/` (legacy, untracked) | `.apex/<a>/` | `mv`; keep its `.gitignore` |
| `.claude/LIVE.md`, `.claude/git.md` (legacy) | `.apex/LIVE.md`, `.apex/git.md` | `git mv` |
| the `## Apex Agents` section of `CLAUDE.md` (legacy) | `AGENTS.md` | rewrite from `templates/AGENTS-section.md` |

Then:

1. Run the shared setup in 7: it adds `.apex/tiers.md`, `.gitattributes` and the Claude Code adapter.
2. Rewrite in-repo references to the old paths, searched as in 5.2.
3. Check: `apex where` prints `layout: apex`; `apex list --status all --agent <a>` per role gives the same counts as
   before the move; `apex skill check` exits 0.
4. Commit as in 10, then rewrite each handoff's `checkout`, `branch` and `head` from the main checkout: the move
   itself would otherwise make every handoff stale.

## 7. Shared setup

Blank slate, migrate and the legacy move finish with these; the shared-setup route runs them alone. Paths are for the
`apex` layout.

1. **AGENTS.md.** Take `templates/AGENTS-section.md` as is. If `<root>/AGENTS.md` is missing, create it with
   `# <repo directory name>` and that section. If it already has an `## Apex Agents` heading, replace that section;
   otherwise append it. Never add it twice.
2. **Shared LIVE.md.** Create `<root>/.apex/LIVE.md` from `templates/shared-LIVE.md` if missing.
3. **git.md.** Fill `templates/git.md` and write `<root>/.apex/git.md`. Infer every row first, then ask the operator
   only about the rows still unknown. A row with no source, such as no remote, is written as `none` once the operator
   confirms. If the file exists, keep it and ask only about missing rows.

   | Row | Source |
   |---|---|
   | Commit, Push, Worker merge | the old role cards and memory; ask only when neither states it |
   | Message format, ticket prefix | `git log -50 --format=%s` |
   | Branch naming | `git branch -a` |
   | Remotes | `git remote -v` |
   | PR template | `.github/pull_request_template.md` or `.github/PULL_REQUEST_TEMPLATE/` |

4. **tiers.md.** Create `<root>/.apex/tiers.md` from `templates/tiers.md` if missing, keeping only the harness
   columns the repo uses.
5. **Line endings.** Add the line `/.apex/** text eol=lf` to `<root>/.gitattributes` if it is not there; create the
   file if missing.

### Claude Code adapter

Only when step 1 found Claude Code in use, or the operator asks for it.

1. **CLAUDE.md.** It must import `AGENTS.md`. If `<root>/CLAUDE.md` is missing, create it from
   `templates/CLAUDE-section.md`. Otherwise add the line `@AGENTS.md` once, at the top, and remove a legacy
   `## Apex Agents` section from it, since that section now lives in `AGENTS.md`.
2. **Claude Code settings.** Set `"autoMemoryEnabled": false` in `<root>/.claude/settings.json`. Merge into the
   existing JSON and keep every other key; create the file with `{"autoMemoryEnabled": false}` if missing.

### Legacy layout setup

When the operator asked for the legacy layout (4) and no move: write the shared files under `.claude/` (legacy) as
`.claude/LIVE.md` and `.claude/git.md`, and the section into `CLAUDE.md` (legacy framing) instead of `AGENTS.md`.
Skip `tiers.md` and `.gitattributes`.

## 8. Conformant

A repo is conformant when every signal holds. Report which held and change nothing. Other-home files do not count.
Project memory files left while Apex roles exist are a pending deletion from 5.4 step 4: name it in the report, not
as a failed signal. Paths come from `apex where`.

| Signal | Check |
|---|---|
| Every stateful role has a ledger that reads cleanly | `apex list --status all --agent <a>` exits 0 |
| No foreign layout remains | step 1 found no foreign ledgers, handoffs or role cards |
| The framing file frames sessions | `<root>/<framing>` contains the `## Apex Agents` heading |
| Shared files exist | `<state>/git.md` and `<state>/LIVE.md` exist, and `<state>/tiers.md` in the `apex` layout |
| Notes are private | every stateful role has `<private>/.gitignore` and no tracked `LIVE.md` next to its skill |
| Orchestrator is set up | when one exists: `<ledger>/concurrent` and `<workers>/.gitignore` exist |
| Skills are valid | `apex skill check` exits 0 |
| Line endings | in the `apex` layout, `.gitattributes` holds `/.apex/** text eol=lf` |
| Claude Code adapter | when Claude Code is in use: `CLAUDE.md` holds `@AGENTS.md` (`apex` layout), and `.claude/settings.json` has `"autoMemoryEnabled": false` |

## 9. Private folders

Moves an Apex role's notes from a tracked `LIVE.md` next to its skill into its private folder `<private>`, which
ignores itself. The role's skill, ledger, handoff and `<state>/LIVE.md` stay where they are.

1. `mkdir -p <private>/skills && printf '*\n' > <private>/.gitignore`.
2. Sort every entry of the old `LIVE.md`:

   | Entry | Goes to |
   |---|---|
   | an operator decision that grants something | `RULES.md`, as `- Allowed: <action, target, condition>` |
   | an operator decision phrased as a ban | `RULES.md` as the grant it implies, plus a `Don't:` in a reminder skill if the ban guards a real risk |
   | a trap, a convention, a do or don't, a how-to | `skills/<topic>.md`, lines `Do:`, `Don't:`, `Trap:`, `Good to know:` |
   | a fact about the current state | `LIVE.md` |
   | stale, or already in the ledger or `<state>/LIVE.md` | dropped |

   Rules carry no dates or history, and each covers a kind of work, not one ledger item. An operator limit (one
   heavy process at a time, a timeout per tool call) is not a rule: it goes in a reminder skill as `Don't:` or `Do:`.
   Show the operator the sorted files and wait for approval: only the operator decides rules.
3. `git -C <root> rm -q <old LIVE.md path>`, or `rm` it when untracked.
4. In the role's skill, replace the `Live notes` row with `| Notes, rules, reminders | <private path> |`. The skill
   is the operator's file: this row only, with the operator's approval.
5. Check: `git -C <root> check-ignore -q <private>/LIVE.md` exits 0, `git -C <root> status --porcelain -- <private>`
   prints nothing, and every `- ` line of `RULES.md` starts with `- Allowed: `.

## 10. Commit

The last step, after the operator approves. Load `git-discipline` and commit what this session created and
deleted in the repo, staged by explicit path. A file on the list of files already modified (section 2) holds the
operator's own edits: leave it out, or name it to the operator and stage it only on their yes.

After a migration or a legacy move, write each handoff's `checkout`, `branch` and `head` from the main checkout,
whether or not the operator approved the commit, so the next session's freshness check has values to run on. The
handoffs written here ride in a later commit, per `git-discipline`.
