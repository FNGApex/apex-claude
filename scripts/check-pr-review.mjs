// Dry-runs the workflow skeletons in skills/pr-review/workflow.md against stub agents. It proves the two rules of the
// skill (at most 10 agents at once, medium effort), the capability tiers, the tier-to-model resolution and the A/B
// agreement table hold in the script itself.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const md = readFileSync(new URL('../skills/pr-review/workflow.md', import.meta.url), 'utf8')
assert.ok(!/\b(sonnet|opus|fable|mythos|haiku)\b/i.test(md), 'workflow.md names no model, only tiers')
assert.ok(!/\bmodel"?\s*:\s*'/.test(md), 'workflow.md has no hardcoded model value')
const scripts = [...md.matchAll(/```js\n([\s\S]*?)```/g)].map(m => m[1].replace('export const meta', 'const meta'))
assert.equal(scripts.length, 2, 'workflow.md holds the review script and the edit script')
const AsyncFunction = (async () => {}).constructor

async function run(script, args, answer) {
  const calls = []
  let running = 0, peak = 0
  const agent = async (prompt, opts) => {
    calls.push({ prompt, ...opts })
    peak = Math.max(peak, ++running)
    await new Promise(r => setTimeout(r, 1))
    running--
    return answer(opts.label, prompt)
  }
  const parallel = thunks => Promise.all(thunks.map(t => t().catch(() => null)))
  const result = await new AsyncFunction('args', 'agent', 'parallel', 'log', script)(args, agent, parallel, () => {})
  return { result, calls, peak }
}

const finding = n => ({ path: 'a.go', line: n, category: 'ISSUE', title: `t${n}`, detail: 'd', fix: 'f' })
const verdict = (id, verdict, category = 'ISSUE') => ({ id, verdict, category, evidence: 'e' })
const ruling = (id, action, category = 'ISSUE') => ({ id, action, category, note: 'n' })
const slices = Array.from({ length: 10 }, (_, i) => ({ name: `s${i}`, files: [`f${i}.go`] }))
const models = { standard: 'm-std', deep: 'm-deep', audit: 'm-audit', security: 'm-sec' }
const args = {
  pr: 'https://example.test/pr/1', dir: '/co', diffPath: '/d.patch', promptDir: '/p', models,
  lenses: ['lean', 'general'], slices, existing: [{ id: 'x-1', author: 'bot', path: null, line: null, body: 'b' }],
}

// One reviewer finds five things; the rest find nothing. Ids are lean-1 .. lean-5.
const answers = (label) => {
  if (label === 'review:lean:s0') return { findings: [1, 2, 3, 4, 5].map(finding) }
  if (label.startsWith('review:')) return { findings: [] }
  if (label === 'judge:A') return { verdicts: [
    verdict('lean-1', 'confirmed'), verdict('lean-2', 'confirmed'), verdict('lean-3', 'refuted'),
    verdict('lean-4', 'confirmed', 'ASK'), verdict('lean-5', 'confirmed'), verdict('x-1', 'refuted')] }
  if (label === 'judge:B') return { verdicts: [
    verdict('lean-1', 'confirmed'), verdict('lean-2', 'refuted'), verdict('lean-3', 'duplicate'),
    verdict('lean-4', 'confirmed', 'ISSUE'), verdict('lean-5', 'confirmed'), verdict('x-1', 'refuted')] }
  if (label === 'auditor:A') return { missed: ['m'], rulings: [
    ruling('lean-1', 'keep'), ruling('lean-2', 'keep'), ruling('lean-4', 'keep', 'ASK'), ruling('lean-5', 'drop')] }
  if (label === 'auditor:B') return { missed: [], rulings: [
    ruling('lean-1', 'keep'), ruling('lean-2', 'drop'), ruling('lean-4', 'keep', 'ASK'), ruling('lean-5', 'drop')] }
  throw new Error(`unexpected agent ${label}`)
}

const tierOf = label => (label.startsWith('review:security:') ? 'security'
  : label.startsWith('review:') ? 'standard' : label.startsWith('auditor:') ? 'audit' : 'deep')
const ids = list => list.map(f => f.id).sort()

const { result, calls, peak } = await run(scripts[0], args, answers)
assert.equal(calls.length, 24, '10 reviewers per lens for 2 lenses, 2 judges, 2 auditors')
assert.ok(peak <= 10, `at most 10 agents at once, saw ${peak}`)
for (const c of calls) {
  assert.equal(c.effort, 'medium', `${c.label} runs at medium effort`)
  assert.equal(c.tier, tierOf(c.label), `${c.label} runs on its role's tier`)
  assert.equal(c.model, models[c.tier], `${c.label} gets the model mapped to ${c.tier}`)
}
assert.deepEqual([...new Set(calls.map(c => c.tier))].sort(), ['audit', 'deep', 'standard'])
assert.deepEqual(ids(result.final), ['lean-1', 'lean-4'], 'kept by both judges and both auditors')
assert.equal(result.final.find(f => f.id === 'lean-4').category, 'ASK', 'judges split on category: the milder one')
assert.deepEqual(ids(result.needsMaster), ['lean-2'], 'a split goes to the Master Reviewer')
assert.deepEqual(ids(result.dropped), ['lean-3', 'lean-5'], 'refuted by both judges, or dropped by both auditors')
assert.deepEqual(result.existing.map(e => [e.id, e.status]), [['x-1', 'refuted']], 'existing comments are judged, not audited')
assert.deepEqual(result.missed, ['m'])
assert.ok(!result.document.includes('lean-3'), 'refuted findings stay out of the findings document')

// The old lens name still works and yields the same lens.
const alias = await run(scripts[0], { ...args, lenses: ['ponytail', 'general'] }, answers)
assert.ok(alias.calls.some(c => c.label === 'review:lean:s0') && !alias.calls.some(c => c.label.includes('ponytail')))
assert.deepEqual(ids(alias.result.final), ['lean-1', 'lean-4'], 'ponytail is read as lean')

// A tier without a model passes none, so the agent inherits the session's.
for (const none of [undefined, {}, { standard: '', deep: '', audit: '' }]) {
  const bare = await run(scripts[0], { ...args, models: none }, answers)
  assert.equal(bare.calls.length, 24)
  assert.ok(bare.calls.every(c => c.model === undefined && !('model' in c)), 'no mapped model: model is left out')
  assert.deepEqual(ids(bare.result.final), ['lean-1', 'lean-4'])
}
const half = await run(scripts[0], { ...args, models: { deep: 'm-deep' } }, answers)
assert.ok(half.calls.filter(c => c.tier === 'deep').every(c => c.model === 'm-deep'))
assert.ok(half.calls.filter(c => c.tier !== 'deep').every(c => c.model === undefined))

// A judge that dies leaves every finding disputed for the auditors; an auditor that dies leaves them to the Master.
const oneJudge = await run(scripts[0], args, label => (label === 'judge:B' ? null : answers(label)))
assert.ok(!oneJudge.result.document.includes('[confirmed]'), 'nothing is confirmed on one judge alone')
assert.deepEqual(ids(oneJudge.result.dropped), ['lean-3', 'lean-5'], 'one judge alone drops only what it refuted')
const oneAuditor = await run(scripts[0], args, label => (label === 'auditor:B' ? null : answers(label)))
assert.deepEqual(ids(oneAuditor.result.final), [], 'nothing is final on one auditor alone')
assert.equal(oneAuditor.result.needsMaster.length, 4)

// The security lens runs only on the security tier's model, never on another's.
const secArgs = { ...args, lenses: ['security', 'lean'] }
const sec = await run(scripts[0], secArgs, answers)
assert.ok(sec.calls.filter(c => c.label.startsWith('review:security:')).every(c => c.tier === 'security' && c.model === 'm-sec'))
assert.ok(sec.calls.filter(c => c.label.startsWith('review:lean:')).every(c => c.tier === 'standard' && c.model === 'm-std'))
assert.deepEqual(sec.result.skipped, [])

// No security model mapped: the lens is skipped before any reviewer runs.
for (const unmapped of [{ standard: 'm-std', deep: 'm-deep', audit: 'm-audit' }, { ...models, security: '' }, undefined, {}]) {
  const mega = await run(scripts[0], { ...secArgs, models: unmapped }, answers)
  assert.deepEqual(mega.result.skipped, ['security'], 'mega without a security model skips the security lens')
  assert.equal(mega.calls.filter(c => c.label.startsWith('review:security:')).length, 0, 'no security reviewer is started')
  assert.deepEqual(ids(mega.result.final), ['lean-1', 'lean-4'], 'and still reviews the other lenses')
  const only = await run(scripts[0], { ...args, lenses: ['security'], models: unmapped }, answers)
  assert.deepEqual(only.result.skipped, ['security'])
  assert.equal(only.calls.length, 0, 'security mode without a security model starts no agent at all')
}

// Every security reviewer failing also skips the lens, without rerouting it.
const noAnswer = label => (label.startsWith('review:security:') ? null : answers(label))
const megaNo = await run(scripts[0], secArgs, noAnswer)
assert.deepEqual(megaNo.result.skipped, ['security'])
assert.deepEqual(ids(megaNo.result.final), ['lean-1', 'lean-4'])
const secNo = await run(scripts[0], { ...args, lenses: ['security'] }, noAnswer)
assert.deepEqual(secNo.result.skipped, ['security'])
assert.ok(!secNo.calls.some(c => c.label.startsWith('judge:')), 'security mode without its reviewers judges nothing')

await assert.rejects(run(scripts[0], { ...args, slices: [...slices, slices[0]] }, answers), /merge them down to 10/)

const edit = await run(scripts[1], { promptDir: '/p', reviewPath: '/r.json', models }, () => ({ body: 'b', comments: [], notes: [] }))
assert.deepEqual(edit.calls.map(c => [c.tier, c.model, c.effort]), [['deep', 'm-deep', 'medium']])
assert.equal(edit.result.body, 'b')
const editBare = await run(scripts[1], { promptDir: '/p', reviewPath: '/r.json' }, () => ({ body: 'b', comments: [], notes: [] }))
assert.ok(editBare.calls.every(c => c.tier === 'deep' && !('model' in c) && c.effort === 'medium'))
console.log('pr-review workflow skeletons: ok')
