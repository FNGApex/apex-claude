import { atom, read, update } from 'claude-code'
import type { Register, SessionContextUsage, SessionRateLimit } from 'claude-code'

const label = atom({ plugin: 'usage-footer', key: 'label' } as const, null)

// Each window shows only when the plan reports it; the Fable window only on plans with a weekly one.
const WINDOWS = [
  ['five_hour', '5h'],
  ['seven_day', 'weekly'],
  ['seven_day_overage_included', 'Fable'],
] as const
const num = new Intl.NumberFormat('en', { notation: 'compact' }).format

export const format = (context: SessionContextUsage, limits: SessionRateLimit[]) => {
  const weekly = limits.some(l => l.kind === 'seven_day')
  const parts: string[] = []
  for (const [kind, name] of WINDOWS) {
    const l = limits.find(l => l.kind === kind)
    if (l && (kind !== 'seven_day_overage_included' || weekly))
      parts.push(`${name} ${Math.max(0, Math.round(100 - l.percentUsed))}% left`)
  }
  if (context.tokens !== undefined) parts.push(`ctx ${num(context.tokens)}/${num(context.window)}`)
  return parts.length ? parts.join(' · ') : null
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const u = await $.session.usage()
    await update($, label, () => format(u.context, u.rateLimits))
    return next(e)
  })

  on('session.measure', async ($, e, next) => {
    await update($, label, () => format(e.context, e.rateLimits))
    return next(e)
  })

  on('ui.render', { component: 'SessionMode' }, async ($, e, next) => {
    const text = await read($, label)
    if (!text) return next(e)
    return next({ ...e, props: { ...e.props, modes: [...e.props.modes, text] } })
  })
}
