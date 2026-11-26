---
name: review
description: "Review a worker branch or diff read-only for correctness, safety, scope and needless code, and return ranked findings."
metadata:
  kind: worker-skill
  tier: deep
  tags: "review, audit, diff, correctness, quality"
  inputs: "branch or diff range, goal of the change, verify command"
  outputs: "ranked findings in the completion report; no commits"
  verify: "every finding cites a file and line and was checked against the code"
---

# Review

Run inside a worker that follows `worker-protocol`. Read-only: change no file and make no commit.

1. Get the change: `git log --oneline <base>..HEAD` and `git diff <base>...HEAD`.
2. Read the connected code, not only the diff: callers, tests, config.
3. Check, in order: does it meet the Goal; is it correct on edge cases; is it safe at trust boundaries; is new risky
   logic tested; is every line needed.
4. Run the Verify command and note its result.
5. Drop any finding you could not confirm in the code.

Report each finding under Notes, most severe first: severity (`issue`, `ask`, `nit`), `file:line`, what is wrong, the
input that breaks it, the smallest fix. No findings: say so.
