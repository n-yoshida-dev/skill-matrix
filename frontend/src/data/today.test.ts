import { describe, expect, it } from 'vitest'
import { resolveToday } from './today'

// 「何を保証しているか」：
// - site.fixedToday が無ければ、渡した今日をそのまま使う（本物の画面は今までどおり）
// - site.fixedToday があれば、その日の 0 時を今日にする（見本の画面の鮮度が日が経っても変わらない）
// - 形の崩れた日付・暦に無い日付は例外にする（黙って本当の今日に戻らない）

const now = new Date(2027, 2, 1, 15, 30)

describe('鮮度を測る今日', () => {
  it('fixedToday が無ければ now', () => {
    expect(resolveToday({}, now)).toBe(now)
    expect(resolveToday({ site: { repoUrl: 'https://example.com' } }, now)).toBe(now)
  })

  it('fixedToday があればその日の 0 時', () => {
    const d = resolveToday({ site: { fixedToday: '2026-10-25' } }, now)
    expect([d.getFullYear(), d.getMonth() + 1, d.getDate(), d.getHours()]).toEqual([
      2026, 10, 25, 0,
    ])
  })

  it('形の崩れた日付は例外', () => {
    expect(() => resolveToday({ site: { fixedToday: '2026/10/25' } }, now)).toThrow(/fixedToday/)
  })

  it('暦に無い日付は例外', () => {
    expect(() => resolveToday({ site: { fixedToday: '2026-02-30' } }, now)).toThrow(/2026-02-30/)
  })
})
