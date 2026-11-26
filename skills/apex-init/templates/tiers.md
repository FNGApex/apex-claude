# Capability tiers

Read by the Orchestrator before it delegates. Roles and worker skills ask for a tier; this file maps it to a model per
harness. A cell left `default` runs on the harness's default model. Keep only the harness columns this repo uses.

| Tier | For | Claude Code | Codex | Other |
|---|---|---|---|---|
| `fast` | search, lookups, mechanical edits | `haiku` | default | default |
| `standard` | most implementation and review work | `sonnet` | default | default |
| `deep` | design, hard debugging, judging | `opus` | default | default |
| `audit` | an independent second check | `fable` | default | default |
| `security` | security review | `mythos` | none | none |

Claude Code takes the alias as the delegate's model, so each tier follows the latest model of its family. `none`
means no model fits: a role that needs the tier stops and says so.
