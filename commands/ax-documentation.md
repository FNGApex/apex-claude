---
description: Bootstrap and maintain documentation surfaces. Two modes — bootstrap (discover doc files, index them in CLAUDE.md) and authoring (scan for unindexed docs, match the diff against indexed surfaces, walk stale/incomplete/missing items). Invokes the ax-documentation skill.
---

<flow>
1. **Mode detect.** No `## Documentation surfaces` table in CLAUDE instructions → **bootstrap**: discover doc files (`README.md`, `docs/**`, package docstrings), propose an indexed table, write it to CLAUDE.md on approval.
2. **Authoring** (table exists): invoke the `ax-documentation` skill in authoring mode. Scan for unindexed docs; match the working/branch diff against indexed surfaces.
3. **Walk items.** For each stale/incomplete/missing surface: Yes (edit now) / Later (followup) / Remind / Skip. Prose-heavy bodies → `ax-explainer` via `ax-writer`.
4. **Verify.** `apex docs stale` → report freshness. Stage edited docs; leave commit to a ship verb.
</flow>

<boundary>
Surface impact + surgical edits = this command. Narrative voice = ax-explainer. Diff-driven maintenance also runs automatically inside ship verbs.
</boundary>
