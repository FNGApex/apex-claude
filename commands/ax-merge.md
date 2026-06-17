---
description: Merge the current branch into base. No squash. Re-runs tests on the merged tip. Detects worktree provenance and prompts to delete. Prefers gh pr merge when a PR is open.
---

<flow>
1. **Pre-flight.** Working tree clean (commit or stash first). Identify base (main/master).
2. **Open PR?** If `gh pr view` finds an open PR for this branch, prefer `gh pr merge --merge` so GitHub closes it cleanly. Else local merge.
3. **Local merge.** `git checkout <base>`, `git merge --no-ff <branch>`.
4. **Verify merged tip.** Re-run the suite (`make test` / project test command). On failure, stop and report — do not push a broken merge.
5. **Worktree provenance.** If the branch lived in `.worktrees/<branch>/`, prompt to remove it.
6. **Report.** Merge commit + test result. Push only if the user asks.
</flow>

<safety>
Merging into base is hard to reverse for collaborators. Use `git revert` for rollback, never force-push base. Confirm before merging if tests did not pass.
</safety>
