import { describe, expect, it } from 'vitest'
import { DEFAULT_LOOK, LOOK_FONTS, LOOKS, lookForPath, lookFromSearch } from './look'

// 見た目の案の当て方（look.ts）を確かめる。
// 「何を保証しているか」：
// - URL の ?look= で案を選べる。知らない名前や指定なしは既定の案
// - 案を当てるのは作業ビュー（/plan とその下）だけ。公開ビューには当てない
// - どの案にも、読み込む書体の設定（無ければ null）がある

describe('見た目の案', () => {
  it('URL の ?look= で案を選び、知らない名前や指定なしは既定の案', () => {
    expect(lookFromSearch('?look=book')).toBe('book')
    expect(lookFromSearch('?look=window&x=1')).toBe('window')
    expect(lookFromSearch('?look=neon')).toBe(DEFAULT_LOOK)
    expect(lookFromSearch('')).toBe(DEFAULT_LOOK)
  })

  it('作業ビューのときだけ案を当てる', () => {
    expect(lookForPath('/plan', 'board')).toBe('board')
    expect(lookForPath('/plan/path/go', 'board')).toBe('board')
    expect(lookForPath('/', 'board')).toBeNull()
    expect(lookForPath('/planner', 'board')).toBeNull()
  })

  it('どの案にも書体の設定がある', () => {
    for (const l of LOOKS) expect(LOOK_FONTS).toHaveProperty(l)
  })
})
