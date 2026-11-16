import type { Register, EngineInterface, ModelUsage } from 'claude-code'

type Tokens = { input: number; output: number; cacheRead: number; cacheWrite: number }
// models and sources are absent on rows stored before those splits; sessions only on the archive row
type Spend = Tokens & {
  usd: number
  at: number
  models?: Record<string, Tokens>
  sources?: Record<string, Tokens>
  sessions?: number
}
type Source = 'main' | 'subagent' | 'teammate' | 'workflow' | 'compaction' | 'other'

const NONE: Tokens = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 }
const ZERO: Spend = { ...NONE, usd: 0, at: 0 }
const ARCHIVE = 'archive'
const ARCHIVE_AFTER_MS = 30 * 24 * 3600 * 1000

const add = (a: Tokens, b: Tokens): Tokens => ({
  input: a.input + b.input,
  output: a.output + b.output,
  cacheRead: a.cacheRead + b.cacheRead,
  cacheWrite: a.cacheWrite + b.cacheWrite,
})
const merge = (into: Record<string, Tokens>, from: Record<string, Tokens> = {}) => {
  for (const [k, t] of Object.entries(from)) into[k] = add(into[k] ?? NONE, t)
  return into
}
const fold = (into: Spend, s: Spend) => {
  Object.assign(into, add(into, s))
  into.usd += s.usd
  into.models = merge(into.models ?? {}, s.models)
  into.sources = merge(into.sources ?? {}, s.sources)
  return into
}
const tokens = (u: ModelUsage): Tokens => ({
  input: u.input_tokens,
  output: u.output_tokens,
  cacheRead: u.cache_read_input_tokens,
  cacheWrite: u.cache_creation_input_tokens,
})

const fmt = (n: number) => n.toLocaleString('en-US')
const line = (t: Tokens) =>
  `input ${fmt(t.input)} · output ${fmt(t.output)} · cache read ${fmt(t.cacheRead)} · cache write ${fmt(t.cacheWrite)}`
const section = (title: string, m: Record<string, Tokens>) => {
  const rows = Object.entries(m).sort(([, a], [, b]) => b.output - a.output)
  return rows.length ? [`  by ${title}:`, ...rows.map(([k, t]) => `    ${k}: ${line(t)}`)] : []
}
const report = (s: Spend) => [`  total: ${line(s)}`, ...section('model', s.models ?? {}), ...section('source', s.sources ?? {})]

// ponytail: agent kinds live in module memory, so agents started before a
// reload count as 'other'; move to $.state if reloads mid-run matter.
const kinds = new Map<string, Source>()
// Parallel agents finish together; serialize the store's read-modify-write so no response is lost.
let queue: Promise<unknown> = Promise.resolve()
function serial<T>(fn: () => Promise<T>): Promise<T> {
  const run = queue.then(fn)
  queue = run.catch(() => {})
  return run
}

// Unknown ids: a forked skill is in $.agent.list(); engine forks and workflow
// retries are not (the engine raises no agent.spawn for either), so they are 'other'.
async function sourceOf($: EngineInterface, agentId?: string): Promise<Source> {
  if (!agentId) return 'main'
  const known = kinds.get(agentId)
  if (known) return known
  const kind = (await $.agent.list()).some(a => a.id === agentId) ? 'subagent' : 'other'
  kinds.set(agentId, kind)
  return kind
}

function record($: EngineInterface, t: Tokens, model: string, source: Source) {
  return serial(async () => {
    const key = `s:${await $.session.id()}`
    const prev = ((await $.store.get(key)) as Spend | undefined) ?? ZERO
    // cost is the session's running total, so keep the latest rather than add
    const usd = (await $.session.usage()).cost?.usd ?? prev.usd
    await $.store.set(key, {
      ...add(prev, t),
      usd: Math.max(usd, prev.usd),
      at: await $.clock.now(),
      models: merge({ ...prev.models }, { [model]: t }),
      sources: merge({ ...prev.sources }, { [source]: t }),
    } satisfies Spend)
  })
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const now = await $.clock.now()
    if (typeof (await $.store.get('installedAt')) !== 'number') await $.store.set('installedAt', now)
    await $.command.register({ name: 'spend', description: 'Token and dollar spend since token-spend was installed' })
    // Fold sessions idle for 30 days into one archive row, so the store stays far under its 4 MiB cap.
    await serial(async () => {
      const old: string[] = []
      const archive = ((await $.store.get(ARCHIVE)) as Spend | undefined) ?? { ...ZERO, sessions: 0 }
      for (const k of (await $.store.keys()).filter(k => k.startsWith('s:'))) {
        const s = (await $.store.get(k)) as Spend
        if (now - s.at < ARCHIVE_AFTER_MS) continue
        fold(archive, s)
        archive.sessions = (archive.sessions ?? 0) + 1
        old.push(k)
      }
      if (!old.length) return
      await $.store.set(ARCHIVE, archive)
      for (const k of old) await $.store.delete(k)
    })
    return next(e)
  })

  on('agent.spawn', async ($, e, next) => {
    const r = await next(e)
    const id = r.agentId ?? e.resume?.agentId
    // An agent started inside a workflow agent is the workflow's spend too.
    const parent = e.parentAgentId ? kinds.get(e.parentAgentId) : undefined
    if (id) kinds.set(id, e.workflow || parent === 'workflow' ? 'workflow' : e.isTeammate ? 'teammate' : 'subagent')
    return r
  })

  // One model response: counted under the model that actually answered it.
  on('turn.step', async function* ($, e, next) {
    const r = yield* next(e)
    if (r.usage) await record($, tokens(r.usage), r.usage.model, await sourceOf($, e.agentId))
    return r
  })

  // The summarizer's request is reported here, never on a turn.
  on('session.compact', async ($, e, next) => {
    const r = await next(e)
    if ('usage' in r && r.usage) await record($, tokens(r.usage), 'compaction', 'compaction')
    return r
  })

  on('command.run', { command: 'spend' }, async $ => {
    const keys = (await $.store.keys()).filter(k => k.startsWith('s:'))
    const rows = (await Promise.all(keys.map(k => $.store.get(k)))) as Spend[]
    const archive = (await $.store.get(ARCHIVE)) as Spend | undefined
    const all = [...rows, ...(archive ? [archive] : [])].reduce(fold, { ...ZERO })
    const count = rows.length + (archive?.sessions ?? 0)
    const me = rows[keys.indexOf(`s:${await $.session.id()}`)] ?? ZERO
    const since = new Date(Number(await $.store.get('installedAt'))).toISOString().slice(0, 10)
    return {
      text: [
        `Since ${since}, ${count} sessions: $${all.usd.toFixed(2)}`,
        ...report(all),
        `This session: $${me.usd.toFixed(2)}`,
        ...report(me),
      ].join('\n'),
    }
  })
}
