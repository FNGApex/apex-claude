---
description: Squash all branch commits into base via git merge --squash. One clean commit on base. Re-runs tests. Detects worktree, prompts to delete. Prefers gh pr merge --squash when a PR is open.
---

<flow>
1. **Pre-flight.** Working tree clean. Identify base.
2. **Open PR?** If an open PR exists, prefer `gh pr merge --squash` (GitHub squashes + closes).
3. **Local squash.** `git checkout <base>`, `git merge --squash <branch>`. Stage is now loaded with the combined diff.
4. **Message.** Invoke `ax-commit` to synthesize one message covering the whole branch. Commit.
5. **Verify.** Re-run the suite on the squashed tip. On failure, stop and report.
6. **Worktree.** If branch came from `.worktrees/`, prompt to remove. Push only if asked.
</flow>

<safety>
Squash rewrites the branch's commit history into one. Confirm the user wants history collapsed before proceeding. Never force-push base.
</safety>
