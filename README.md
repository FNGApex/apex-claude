# Workflow Protocol

A personal Claude Code agent layer, packaged as a plugin. It encodes a coding loop —
plan, build with tests, review, ship — across the mechanisms Claude Code exposes for
customization.

This is a skeleton meant to be grown, not a finished system. Each piece is intentionally
small so you can see how the parts connect before adding your own.

## What's here

| Path | Mechanism | Job |
|------|-----------|-----|
| `.claude-plugin/plugin.json` | Plugin manifest | Names and versions the bundle |
| `output-styles/protocol.md` | Output style | How Claude talks: signal-first, terse |
| `agents/wp-investigator.md` | Subagent (haiku, read-only) | Locate code, return `file:line` tables |
| `agents/wp-builder.md` | Subagent (sonnet) | Implement one slice, test-first |
| `agents/wp-reviewer.md` | Subagent (sonnet, read-only) | Review a diff, emit a `VERDICT:` line |
| `skills/wp-tdd/SKILL.md` | Skill | Test-first discipline, auto-triggers |
| `skills/wp-commit/SKILL.md` | Skill | Conventional Commits message generator |
| `commands/wp-ship.md` | Slash command | Review the diff, gate on verdict, then commit |
| `hooks/hooks.json` | Hook | Wires the `PreToolUse(Bash)` guard to the compiled CLI |
| `cmd/wp/` → `bin/wp` | Go CLI | Deterministic backbone: the hook guard + `doctor` |

## How the pieces relate

The command orchestrates; skills carry policy; agents do isolated work; the hook enforces.

```
/wp-ship  ──dispatches──►  wp-reviewer  ──VERDICT──►  gate
   │                                                   │ PASS
   └──uses skill──►  wp-commit  ◄──────────────────────┘
hooks  ──run regardless of the model──►  block destructive Bash
```

The split that matters: the output style, skills, and agents are *contextual* — Claude
reads them and usually complies. The hook is *deterministic* — it runs no matter what the
model decides, and it can block. Put anything that must happen in the hook layer; put
judgment and voice everywhere else.

The deterministic work itself lives in a compiled Go CLI (`bin/wp`), not in shell or
markdown. A single static binary with zero runtime dependencies: it starts in
milliseconds (so it can sit in a hook), cross-compiles to every platform, and returns
real exit codes that commands branch on. The markdown owns judgment; the binary owns
determinism.

## Try it

This loads in place via the skills directory — no install step:

```
claude --skills-dir /home/bear/GitHub/claudeWorkflowProtocol
```

Then:

- `/config` → Output style → Protocol
- Ask "where is X defined" and watch `wp-investigator` get dispatched
- Make a change, then run `/wp-ship`

To package it for real distribution later, publish to a marketplace and install with
`claude plugin install workflow-protocol@<marketplace>`.

## The CLI

Built with Go (stdlib only, no external dependencies):

```bash
make build      # -> bin/wp (static binary)
make test       # unit tests for the hook guard
make release    # cross-compile bin/<os>-<arch>/wp for mac/linux/windows
```

Subcommands:

```bash
wp hooks pre-bash   # PreToolUse(Bash) guard; reads hook JSON on stdin
wp doctor           # integrity check on the plugin layout (exit 1 on failure)
wp version
```

Test the guard directly:

```bash
echo '{"tool_input":{"command":"rm -rf ~"}}' | bin/wp hooks pre-bash; echo "exit=$?"
# exit=2 with a deny payload

echo '{"tool_input":{"command":"npm test"}}' | bin/wp hooks pre-bash; echo "exit=$?"
# exit=0, no output (allowed)
```

The deny list is deliberately narrow: `rm -rf` on root/home, `--force` push to
main/master, and piping a remote script into a shell. Widen it in `cmd/wp/hooks.go`
(and add a case to `hooks_test.go`) as you learn which mistakes you actually make.

> The compiled binary is gitignored. Run `make build` after cloning so the hook has
> something to call; ship per-platform binaries via `make release` + GitHub Releases.

## Where to grow next

- A `wp-plan` command that writes a brief, then drives `wp-builder` → `wp-reviewer` in a loop.
- A `SessionStart` hook (`wp hooks session-start`) that injects project context.
- More `wp` subcommands for deterministic work — signal scans, staleness checks,
  a follow-ups ledger — each returning exit codes the commands branch on.
