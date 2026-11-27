# Workflow for one pull request

The Master Reviewer authors a workflow for each pull request from the two skeletons below. Neither is ever saved into
the repo. Each runs one of two ways: through a harness's workflow runner, which executes the script, or by hand, with
the Manual run table.

What you may change per pull request: the slices, their `focus`, the lenses and `models`, all through `args`. What you
never change: `TIER`, `EFFORT`, `CAP`, and the A/B agreement rules.

## Runtime contract

The script is plain JavaScript that runs as the body of an async function. The runner provides:

| Name | Is |
|---|---|
| `args` | the input object described below |
| `agent(prompt, opts)` | runs one agent and resolves to its structured answer, or `null` when it fails; `opts` carries `label`, `phase`, `tier`, optionally `model`, `effort` and `schema` |
| `parallel(thunks)` | runs the thunks concurrently and resolves to their answers, a failed one as `null` |
| `log(text)` | notes something for the Master Reviewer |

No script names a model. A role names a capability `tier`; `args.models` maps a tier to the operator's model from
`tiers.md`. A tier with no model leaves `model` out of the call, and the agent inherits the session's model.

## Manual run

Without a workflow runner, do what the script does, in the same order, with your harness's subagents or delegates. Run
the combine rules in the script's code (the votes, the category rule, the A/B table below) by hand on the answers.

| Step | Do |
|---|---|
| Review | for each lens in order, start one subagent per slice, at most 10 at a time, on the lens's tier (`standard`, or `security` for the security lens) at `medium` effort, with the reviewer prompt in the script; number findings `<lens>-<n>` |
| Skip | a security lens with no mapped `security` model is not run and is listed in `skipped`; so is a lens whose every reviewer failed |
| Judge | two subagents, A and B, on `deep`, each with the candidates and the judge prompt; they never see each other's answer |
| Audit | two subagents, A and B, on `audit`, each with the findings document and the auditor prompt |
| Edit | one subagent on `deep` with the edit script's prompt |

Every answer follows the schema in the script. Pass a subagent its tier's model when `models` maps one; otherwise let it
inherit.

## Review script

`args`:

| Key | Value |
|---|---|
| `pr` | the pull request's URL |
| `dir` | absolute path of the checkout holding the pull request |
| `diffPath` | absolute path of `diff.patch` |
| `promptDir` | absolute path of this skill's `prompts/` directory |
| `lenses` | the mode's lenses in order, for example `["security", "lean", "general"]`; `ponytail` is read as `lean` |
| `slices` | `[{name, files, focus}]`, at most 10 |
| `existing` | the contents of `existing.json` |
| `models` | optional `{standard, deep, audit, security}`: the model of each tier that `tiers.md` maps |

```js
export const meta = {
  name: 'pr-review',
  description: 'Review one pull request: reviewers per slice, A/B judges, A/B auditors',
  phases: [
    { title: 'Review', detail: 'one reviewer per slice, lens by lens; the security tier for the security lens' },
    { title: 'Judge', detail: 'A and B verify every candidate and existing comment' },
    { title: 'Audit', detail: 'A and B rule on the findings document' },
  ],
}

const TIER = { reviewer: 'standard', security: 'security', judge: 'deep', auditor: 'audit', editor: 'deep' }
const EFFORT = 'medium'
const CAP = 10
const RANK = ['NIT', 'LOW', 'ASK', 'ISSUE']
const CATEGORY = { type: 'string', enum: RANK }
const FINDINGS = {
  type: 'object',
  properties: { findings: { type: 'array', items: {
    type: 'object',
    properties: {
      path: { type: 'string' }, line: { type: 'integer' }, category: CATEGORY,
      title: { type: 'string' }, detail: { type: 'string' }, fix: { type: 'string' },
    },
    required: ['path', 'line', 'category', 'title', 'detail', 'fix'],
  } } },
  required: ['findings'],
}
const VERDICTS = {
  type: 'object',
  properties: { verdicts: { type: 'array', items: {
    type: 'object',
    properties: {
      id: { type: 'string' }, verdict: { type: 'string', enum: ['confirmed', 'refuted', 'duplicate'] },
      category: CATEGORY, evidence: { type: 'string' },
    },
    required: ['id', 'verdict', 'category', 'evidence'],
  } } },
  required: ['verdicts'],
}
const RULINGS = {
  type: 'object',
  properties: {
    rulings: { type: 'array', items: {
      type: 'object',
      properties: {
        id: { type: 'string' }, action: { type: 'string', enum: ['keep', 'drop'] },
        category: CATEGORY, note: { type: 'string' },
      },
      required: ['id', 'action', 'category', 'note'],
    } },
    missed: { type: 'array', items: { type: 'string' } },
  },
  required: ['rulings', 'missed'],
}

const { pr, dir, diffPath, promptDir, slices, existing, models } = args
// `ponytail` is the old name of the `lean` lens.
const lenses = [...new Set(args.lenses.map(l => (l === 'ponytail' ? 'lean' : l)))]
if (slices.length > CAP) throw new Error(`${slices.length} slices; merge them down to ${CAP}`)
const inputs = `Pull request: ${pr}\nCheckout (read-only): ${dir}\nDiff: ${diffPath}`
// A tier without a mapped model passes no model: the agent inherits the session's.
const run = (tier, prompt, opts) => {
  const model = models?.[tier]
  return agent(prompt, { ...opts, tier, ...(model ? { model } : {}), effort: EFFORT })
}
const pair = (role, phase, prompt, schema) => parallel(['A', 'B'].map(n => () =>
  run(TIER[role], prompt, { label: `${role}:${n}`, phase, schema })))
const byId = (answer, key) => new Map((answer ? answer[key] : []).map(v => [v.id, v]))

// Lenses run one after another, so at most slices.length reviewers are ever running.
const candidates = [], skipped = []
for (const lens of lenses) {
  const tier = lens === 'security' ? TIER.security : TIER.reviewer
  // The security lens never runs on another tier's model: without its own, skip it.
  if (tier === TIER.security && !models?.[tier]) {
    log(`no ${tier} tier model is configured in tiers.md; ${lens} lens skipped`)
    skipped.push(lens)
    continue
  }
  const answers = await parallel(slices.map(s => () => run(tier,
    `Read ${promptDir}/reviewer.md and ${promptDir}/lens-${lens}.md and follow them.\n${inputs}\n` +
    `Your slice: ${s.name}\nFiles:\n${s.files.join('\n')}${s.focus ? `\nFocus: ${s.focus}` : ''}`,
    { label: `review:${lens}:${s.name}`, phase: 'Review', schema: FINDINGS })))
  // Every reviewer of the lens failing means its model is unreachable: skip the lens rather than review it on another.
  if (!answers.some(Boolean)) {
    log(`every ${lens} reviewer returned nothing; the ${tier} tier model may be unavailable, lens skipped`)
    skipped.push(lens)
    continue
  }
  answers.forEach((a, i) => {
    if (!a) return log(`reviewer ${lens}:${slices[i].name} returned nothing; that slice is unreviewed for this lens`)
    for (const f of a.findings) candidates.push({ ...f, id: `${lens}-${candidates.length + 1}`, lens, source: 'reviewer' })
  })
}
const empty = { final: [], needsMaster: [], dropped: [], existing: [], missed: [], document: '', skipped }
if (skipped.length === lenses.length) return empty
for (const e of existing) candidates.push({ ...e, source: 'existing' })
if (!candidates.length) return empty

const [ja, jb] = (await pair('judge', 'Judge',
  `Read ${promptDir}/judge.md and follow it.\n${inputs}\nCandidates:\n${JSON.stringify(candidates, null, 1)}`,
  VERDICTS)).map(a => byId(a, 'verdicts'))
const judged = candidates.map(c => {
  const votes = [ja.get(c.id), jb.get(c.id)]
  const yes = votes.filter(v => v && v.verdict === 'confirmed')
  // Two judges who confirm but disagree on the category get the milder one; the auditors can raise it.
  const category = yes.length ? RANK[Math.min(...yes.map(v => RANK.indexOf(v.category)))] : c.category
  return { ...c, category, status: ['refuted', 'disputed', 'confirmed'][yes.length],
    judgeA: votes[0] ? `${votes[0].verdict}: ${votes[0].evidence}` : 'no verdict',
    judgeB: votes[1] ? `${votes[1].verdict}: ${votes[1].evidence}` : 'no verdict' }
})
const theirs = judged.filter(c => c.source === 'existing')
const ours = judged.filter(c => c.source === 'reviewer' && c.status !== 'refuted')
const dropped = judged.filter(c => c.source === 'reviewer' && c.status === 'refuted')

const document = [
  `# Findings: ${pr}`, '',
  ...ours.map(f => [
    `## ${f.id} [${f.status}] ${f.category}: ${f.title}`, `${f.path}:${f.line} (lens: ${f.lens})`, '',
    f.detail, '', `Fix: ${f.fix}`, '', `Judge A: ${f.judgeA}`, `Judge B: ${f.judgeB}`, '',
  ].join('\n')),
  '# Existing comments', '',
  ...theirs.map(e => `- ${e.id} [${e.status}] ${e.author}${e.path ? ` ${e.path}:${e.line}` : ''}: ` +
    `${String(e.body).slice(0, 300)}\n  Judge A: ${e.judgeA}\n  Judge B: ${e.judgeB}`),
].join('\n')
if (!ours.length) return { ...empty, dropped, existing: theirs, document }

const audits = await pair('auditor', 'Audit',
  `Read ${promptDir}/auditor.md and follow it.\n${inputs}\nFindings document:\n${document}`, RULINGS)
const [ra, rb] = audits.map(a => byId(a, 'rulings'))
const final = [], needsMaster = []
for (const f of ours) {
  const a = ra.get(f.id), b = rb.get(f.id)
  const row = { ...f, auditorA: a ? `${a.action} ${a.category}: ${a.note}` : 'no ruling',
    auditorB: b ? `${b.action} ${b.category}: ${b.note}` : 'no ruling' }
  if (a && b && a.action === 'drop' && b.action === 'drop') dropped.push(row)
  else if (a && b && a.action === 'keep' && b.action === 'keep' && a.category === b.category) final.push({ ...row, category: a.category })
  else needsMaster.push(row)
}
return { final, needsMaster, dropped, existing: theirs, missed: audits.filter(Boolean).flatMap(a => a.missed), document,
  skipped }
```

How A and B combine:

| Stage | Both agree to keep | Both agree to drop | They differ |
|---|---|---|---|
| Judges | `confirmed` | `refuted`, dropped | `disputed`, sent on to the auditors |
| Auditors | `final`, when the category matches too | dropped | `needsMaster`: you decide |

An agent that returns nothing counts as a disagreement. With one judge missing, nothing is `confirmed` and the
auditors settle every finding; with one auditor missing, every finding comes to you.

## Edit script

Run after `review.json` is written. `args`: `promptDir`, `reviewPath` (the absolute path of `review.json`) and the same
`models`.

```js
export const meta = {
  name: 'pr-review-edit',
  description: 'Check the drafted pull request review for formatting and tone',
  phases: [{ title: 'Edit', detail: 'one editor reads the draft review' }],
}

const TIER = { editor: 'deep' }
const REVIEW = {
  type: 'object',
  properties: {
    body: { type: 'string' },
    comments: { type: 'array', items: {
      type: 'object',
      properties: { path: { type: 'string' }, line: { type: 'integer' }, body: { type: 'string' } },
      required: ['path', 'line', 'body'],
    } },
    notes: { type: 'array', items: { type: 'string' } },
  },
  required: ['body', 'comments', 'notes'],
}
const model = args.models?.[TIER.editor]
return await agent(
  `Read ${args.promptDir}/editor.md and follow it.\nDraft review: ${args.reviewPath}`,
  { label: 'editor', phase: 'Edit', tier: TIER.editor, ...(model ? { model } : {}), effort: 'medium', schema: REVIEW })
```
