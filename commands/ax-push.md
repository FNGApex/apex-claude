---
description: Commit the current changes, then push. No PR, no merge. Delegates message format to the ax-commit skill.
---

<flow>
1. **Pre-flight.** `git status --short`. If clean, stop.
2. **Branch guard.** If on the base branch (main/master), stop and offer to branch first. Never push straight to base.
3. **Stage + message.** Stage intended files by path. Invoke `ax-commit` for the message; commit.
4. **Push.** `git push` (set upstream with `-u` if the branch has no remote tracking ref).
5. **Report.** Commit hash + remote ref. Do not open a PR.
</flow>

<safety>
- Stage explicitly by path — never `git add -A`. Skip secrets and build artifacts.
- Pushing publishes to the remote: it may be cached or indexed. Confirm intent if unsure.
</safety>
