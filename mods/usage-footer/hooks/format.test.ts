import { expect, test } from 'claude-code/testing'
import { format } from './register'

const ctx = { tokens: 82_345, window: 200_000 } as Parameters<typeof format>[0]
const w = (kind: string, percentUsed: number) => ({ kind, percentUsed })

test('shows each reported window; Fable only beside a weekly one', async () => {
  expect(format(ctx, [w('five_hour', 2), w('seven_day', 31)])).toBe('5h 98% left · weekly 69% left · ctx 82K/200K')
  expect(format(ctx, [w('seven_day', 27), w('seven_day_overage_included', 10)])).toBe(
    'weekly 73% left · Fable 90% left · ctx 82K/200K',
  )
  expect(format(ctx, [w('seven_day_overage_included', 10)])).toBe('ctx 82K/200K')
  expect(format({ tokens: 412_000, window: 1_000_000 }, [])).toBe('ctx 412K/1M')
  expect(format({ window: 200_000 }, [])).toBe(null)
})
