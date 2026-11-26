## Apex Agents

- **Start:** invoke the Orchestrator skill, `/orchestrator`, or a Collaboration Agent's skill, `/<agent>`. When the
  harness does not list it, read `.agents/skills/<name>/SKILL.md` and follow it.
- **Sign off:** say "sign off" or run `/sign-off`.
- **Paths:** `apex where` prints them. State lives in `.apex/`, skills in `.agents/skills/`.
- **Workers:** a session spawned by `apex worker spawn` is a stateless worker: it follows `worker-protocol`, stays in
  its worktree and never writes `.apex/` state.

### Cross-Agent communication (Multi-Agent Collaboration)

1. Talk to a peer Agent whenever the work needs it, through the harness's peer-messaging primitive.
2. When a peer asks you for something: with no item in progress, act on it and reply right away; with an item in
   progress, file the request in your own ledger and reply with the new item's id.
3. Keep peer traffic out of the operator's chat. Acknowledge a peer's message in one line and answer the peer; never
   repeat to the operator what you sent to a peer. Bring it to the operator only when you need the operator's answer.
