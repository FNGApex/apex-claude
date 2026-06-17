---
description: Watch CI for the current branch (or a specified target) in the background. Dispatches an ax-haiku runner that inspects project signals to identify the CI system and picks the right CLI. Returns immediately; reports when CI reaches a terminal state.
argument-hint: [branch | run-url]
---

<flow>
1. **Identify CI.** Read project signals for the CI provider (GitHub Actions, GitLab CI, CircleCI…). If none configured, say so and stop.
2. **Dispatch ax-haiku** (background) with a self-contained brief: poll the run for $ARGUMENTS (default current branch), emit on every terminal state (success/failure/cancelled/timeout), and report logs on failure.
3. **Return now.** Tell the user it's watching; the runner reports back on a terminal state.
</flow>

<notes>
The runner is provider-agnostic — the brief embeds the exact CLI commands derived from signals. Read-only.
</notes>
