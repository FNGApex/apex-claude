---
name: implement
description: "Implement one scoped code change on the worker branch: a feature, a fix or a refactor, verified by the brief's command."
metadata:
  kind: worker-skill
  tier: standard
  tags: "code, implement, feature, fix, refactor, change"
  inputs: "goal, scope paths, verify command"
  outputs: "commits on the worker branch, completion report"
  verify: "the brief's Verify command passes and the worktree is clean"
  requires: "git-discipline"
---

# Implement

Run inside a worker that follows `worker-protocol`.

1. Read the code the Goal touches and every caller, test and config the change must reach. That is the scope; stay
   inside the brief's Scope paths.
2. Reuse what the codebase already has before writing new code. Make the smallest change that fully meets the Goal.
3. Add or update one test for new non-trivial logic. Trivial changes need none.
4. Run the Verify command. On failure, fix the cause, not the test, and rerun.
5. Commit by explicit path, one reviewable unit per concern, in `git-discipline` format.
6. `git status --porcelain` prints nothing.

Report Notes: what you left out on purpose, any risk a reviewer should check, follow-ups found.
