# Capability tiers

Read by the Orchestrator before it delegates. Roles and worker skills ask for a tier; this file maps it to a model per
harness. A cell left `default` runs on the harness's default model. Keep only the harness columns this repo uses.

| Tier | For | Claude Code | T3 Code | Codex | Other |
|---|---|---|---|---|---|
| `fast` | search, lookups, mechanical edits | `haiku` | `claudeAgent/claude-haiku-5-5` | default | default |
| `standard` | most implementation and review work | `sonnet` | `claudeAgent/claude-sonnet-5-5 (effort=high)` | default | default |
| `deep` | design, hard debugging, judging | `opus` | `claudeAgent/claude-opus-5-5 (effort=high)` | default | default |
| `audit` | an independent second check | `fable` | `claudeAgent/claude-fable-5-2 (effort=high)` | default | default |
| `security` | security review | `mythos` | none | none | none |

Claude Code takes the alias as the delegate's model, so each tier follows the latest model of its family. T3 Code cells
are `providerInstanceId/model (options)`; it also supports Codex and Gemini providers, so list the providers and
models `orchestrator_capabilities` reports and remap a tier to any of them. `none` means no model fits (`security`
stays `none` on T3 Code until configured): a role that needs the tier stops and says so.
