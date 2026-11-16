---
checkout: /Users/alexanderstroev/Documents/GitHub/apex-claude
branch: master
head: f8cf7f3a4c65613d48b92f435b598b3d5704aa53
---
## Startup
invoke the Agent skill (reads its private folder and .claude/LIVE.md); apex list --agent main

## State
No item in progress. 012 dropped (operator rejected per-Agent themes). 013 closed: token-spend ships in mods/ as
token-spend@apex, installed on this machine with its /spend history merged. Uncommitted: mods/, Makefile,
scripts/*.ps1, README.md, docs/design/agents.md, .claude/LIVE.md, ledger 012-013, this handoff. tmp/ is untracked
(holds tmp/token-spend-store-backup.json).

## Next
Nothing queued. Candidate: port usage-footer (context fill, ~/.claude-work/mods/usage-footer, loaded by the
claude-work alias via --plugin-dir) the same way; the operator narrowed 013 to /spend.

## Open threads
- ~/.claude/mods/token-spend and its local-mods marketplace entry, and the obsolete ~/.claude/mods/skill-themes
  prototype, await the operator's word on deletion.
- PowerShell installer edits are untested here (no pwsh); verify on Windows.
