---
name: "{{skill}}"
description: "{{one-line purpose: what kind of work this skill does and when to pick it}}"
metadata:
  kind: worker-skill
  tier: standard
  tags: "{{comma-separated words the Orchestrator searches for}}"
  inputs: "{{what the brief must give}}"
  outputs: "{{what the worker returns besides the report}}"
  verify: "{{how done is proved}}"
  requires: "git-discipline"
---

# {{Skill}}

Run inside a worker that follows `worker-protocol`. Stay inside the brief's Scope.

## Steps

1. {{first step}}
2. {{next step}}
3. Run the brief's Verify command; fix until it passes.

## Report

Fill the `worker-protocol` report. {{what this skill adds under Notes}}
