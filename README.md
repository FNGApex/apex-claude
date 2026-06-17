# Apex Claude

Apex Claude is a Claude Code plugin that encodes a full coding workflow: plan, build with
tests, review, ship, and improve. It pairs a set of agents, skills, slash commands, an output
style, and a hook with a small Go backbone binary (`apex`) that does the deterministic work.

The guiding split runs through everything here. The binary owns determinism: scanning the repo,
linting artifacts, gating on staleness, detecting state, scoring health. The model owns judgment:
classification, drafting, design, and the decision about whether a change is actually correct.
When code can answer a question, code answers it. The model is reserved for the calls that need
reasoning.

## How the layer is organized

| Surface | Mechanism | Job |
|---------|-----------|-----|
| `.claude-plugin/plugin.json` | Plugin manifest | Names and versions the bundle |
| `output-styles/protocol.md` | Output style (Apex) | How Claude talks: article-dropping, signal-first, tiered |
| `agents/ax-*.md` | Subagents | Locate, build, review, reason, plan, debug, write, run |
| `skills/ax-*/SKILL.md` | Skills | Test-first, commit, verify, review, documentation, prose voice |
| `commands/ax-*.md` | Slash commands | The lifecycle verbs, from setup through ship and improve |
| `hooks/hooks.json` | Hooks | PreToolUse(Bash) guard and SessionStart context, wired to the binary |
| `cmd/apex/` to `bin/apex` | Go CLI | The deterministic backbone |

## The lifecycle

The commands follow one loop. Each verb is small and self-describing in the slash listing.

1. **Plan.** `/ax-plan` researches the task across the codebase and online, then writes a design
   doc and a checkpoint spec. Trivial work gets an inline spec instead. `/ax-pressure-test`
   challenges a design before you commit to it.
2. **Implement.** `/ax-implement` runs the implement and review loop: it briefs a fresh-context
   builder, gates each iteration on the reviewer's confidence score, and commits per green pass.
   `/ax-diagnose` covers failure-driven work, starting from a broken test or a symptom.
3. **Ship.** Pick the verb that matches how far you want to go: `/ax-ship`, `/ax-push`,
   `/ax-pr`, `/ax-merge`, or `/ax-squash`.
4. **Sync docs.** `/ax-documentation` keeps the human-facing surfaces current. Ship verbs run
   its maintenance mode automatically.
5. **Improve.** `/ax-improve` mines a session for friction and turns it into concrete fixes.
   `/ax-help` routes you to the right verb when you are unsure.

`/ax-autopilot` runs the whole loop hands-off, with one human decision: how to merge.

## The orchestrator owns the truth

Subagents never claim a task is done. They gather evidence and report it up. The orchestrator,
the main loop you talk to, owns final verification, finds the fix when something is wrong, and
makes every gate decision.

Reviewers and documentation checks reflect this. Instead of a binary pass or fail verdict, they
emit a confidence score from 0 to 100 and flag problems without prescribing the fix. Findings
carry four severity tiers: blocker, risk, nit, and uncertain. The orchestrator aggregates those
scores into a persisted repo health signal that `apex health` reads and `apex doctor` checks.

## Two voices

Claude's replies in the terminal use the Apex output style: article-dropping, structural forms
by default, with intensity tiers (lite, full, ultra) you can switch mid-session. Security
warnings and irreversible-action confirmations always revert to full prose so nothing critical
gets compressed away.

Enduring narrative documentation, like this README and anything under `docs/guides/`, uses the
opposite voice through the `ax-explainer` module: clear, expansive, and detailed. Everything
else, including specs, design docs, and signals files, stays terse and technical.

## The backbone

The deterministic work lives in a single static Go binary built from the standard library with
no external dependencies. It starts in milliseconds, which is what lets it sit inside a hook,
and it returns real exit codes that commands branch on.

```bash
make build      # -> bin/apex (static binary)
make test       # go test ./...
make vet        # go vet ./...
make fmt        # gofmt -w cmd internal
make release    # cross-compile bin/<os>-<arch>/apex for mac, linux, windows
```

The subcommand groups:

```bash
apex signals scan|show|stale     # deterministic project map + staleness gate
apex health show|set             # repo health/integrity score
apex followups list|add|close|render|path
apex reminder add|list|show|rm|due
apex hooks pre-bash|session-start
apex doctor                      # integrity check on the plugin layout + project state
apex validate                    # lint artifacts and specs (exit 1 on issues)
apex docs scan|stale             # documentation-surface cache and staleness gate
apex version
```

Test the Bash guard directly:

```bash
echo '{"tool_input":{"command":"rm -rf ~"}}' | bin/apex hooks pre-bash; echo "exit=$?"
# exit=2 with a deny payload

echo '{"tool_input":{"command":"go test ./..."}}' | bin/apex hooks pre-bash; echo "exit=$?"
# exit=0, no output (allowed)
```

The deny list is deliberately narrow: `rm -rf` on root or home, force-pushing to main or master,
and piping a remote script into a shell. Widen it as you learn which mistakes you actually make.

> The compiled binary is gitignored. Run `make build` after cloning so the hook has something to
> call, and ship per-platform binaries through `make release` plus GitHub Releases.

## Installing and adopting

Enable the plugin through a marketplace, or load it in place during development. After the plugin
is enabled, build the binary so the hooks have something to invoke:

```bash
make build
apex doctor
```

A Claude Code plugin cannot ship an always-on instruction file. The Apex spine, which holds the
principles, the determinism boundary, the lifecycle, and the artifact registries, lives in this
repo's `CLAUDE.md`. To adopt Apex's conventions in another project, copy the spine sections from
`CLAUDE.md` into that project's `.claude/CLAUDE.md`, or into `~/.claude/CLAUDE.md` for user-wide
use. In this repo the spine auto-loads, so Apex builds itself under its own rules.
