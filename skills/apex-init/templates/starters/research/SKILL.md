---
name: research
description: "Answer one question from the codebase, docs and allowed sources, read-only, and return a sourced answer."
metadata:
  kind: worker-skill
  tier: standard
  tags: "research, investigate, question, explore, docs"
  inputs: "the question, where to look, what form the answer takes"
  outputs: "a sourced answer in the completion report; no commits unless the brief asks for a notes file"
  verify: "every claim cites a file, a command output or a source"
---

# Research

Run inside a worker that follows `worker-protocol`. Read-only unless the brief asks for a file.

1. Restate the question in one line and what would settle it.
2. Search narrowly first: the paths the brief names, then the wider repo, then outside sources the brief allows.
3. Read excerpts, not whole trees. Stop once the question is settled.
4. Check each claim against its source; mark what stays uncertain.

Report Summary: the answer first, in a few lines. Notes: each claim with its source (`file:line`, command, link) and
what you could not confirm.
