---
checkout: C:/Users/drgor/Documents/GitHub/apex-claude
branch: generalization
head: 57a8f8c5ed4234e49965b423262d66ea94c5d97e
---
## Startup
invoke the Agent skill (reads its private folder and .claude/LIVE.md); apex list --agent main

## State
No item in progress; 017-023 closed (017-022 from this checkout; 023 merged from origin/master 4453660, where
ServerOverlay was migrated to Agents backend + oauth at 99b66f2 / 3838124 and pushed to its origin). Branch
`generalization` merges `origin/master` (4453660) with the cross-harness orchestration workflow, Go `apex` engine
(layouts, worker spawn/adopt, skill index), parameterized installers (`t3` and `gemini` presets included), and
generalized `MIGRATION.md`. `Apex Overhaul` migrated to the orchestration workflow on `apex/orchestration-migration`
(FNGApex/apex-overhaul#1). No uncommitted work outside untracked `.claude/.scratchpad/` and `.claude/project/`.

## Next
Nothing open. Review and merge `FNGApex/apex-claude#1` (`generalization` -> `master`). Candidates carried forward:
cherry-pick 2a9158e and 5503dfa from backup/pre-fork-reset if the operator wants them; a `.gitattributes` `eol=lf`
guard; migrate `Apex_Tweaks` Agents.

## Open threads
- Operator: keep or cherry-pick the Windows fixes on tag `backup/pre-fork-reset`.
- `usage-footer`'s Fable slot (`seven_day_overage_included`) is untested live: neither plan reports that window today.
- POSIX `sh` / `Makefile` paths run through `sh` in Go tests here (`make` itself is absent on this Windows machine;
  PowerShell 5.1 and `pwsh` 7 paths pass).
