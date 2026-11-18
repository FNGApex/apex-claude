---
checkout: C:/Users/drgor/Documents/GitHub/apex-claude
branch: master
head: 3694c0a1b2de951a246f877ceb157dc61ec09b57
---
## Startup
invoke the Agent skill (reads its private folder and .claude/LIVE.md); apex list --agent main

## State
No item in progress; 017-022 closed this session. master = alexander-xage/master (267779c) plus 8a2d141, 017c4e0,
3694c0a (rules guidance, installer sweeps marked skills, installer passes its config dir to claude plugin). Install
rerun from this tree. Apex Overhaul: all three Agents on private folders (b09b37d, 0ad2e8f, 8e7639f), decompile read
directly (33c5970). No uncommitted work outside .claude/.

## Next
Nothing open. Candidates: cherry-pick 2a9158e and 5503dfa from backup/pre-fork-reset if the operator wants them;
a .gitattributes eol=lf so ledger files cannot check out CRLF; migrate Apex_Tweaks Agents.

## Open threads
- Operator: keep or cherry-pick the Windows fixes on tag backup/pre-fork-reset.
- Makefile changes in 017c4e0 and 3694c0a are untested here (no make on this machine).
