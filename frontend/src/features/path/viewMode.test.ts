import { afterEach, describe, expect, it, vi } from 'vitest'
import { VIEW_MODE_KEY, readViewMode, writeViewMode } from './viewMode'

// 学習パスの表示（一列／ツリー）の記憶（viewMode.ts）を確かめる。
// 「何を保証しているか」：
// - 何も記憶していなければ一列表示
// - ツリーを選ぶと記憶し、次に読むとツリー
// - localStorage が使えない環境（読み書きで例外）でも落ちず、一列表示で開く

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

/** 読み書きのたびに例外を投げる localStorage の代わり */
const broken = {
  getItem: () => {
    throw new Error('使えない')
  },
  setItem: () => {
    throw new Error('使えない')
  },
} as unknown as Storage

describe('学習パスの表示の記憶', () => {
  it('何も記憶していなければ一列表示', () => {
    expect(readViewMode()).toBe('list')
  })

  it('ツリーを選ぶと記憶し、次に読むとツリー', () => {
    writeViewMode('tree')
    expect(localStorage.getItem(VIEW_MODE_KEY)).toBe('tree')
    expect(readViewMode()).toBe('tree')
    writeViewMode('list')
    expect(readViewMode()).toBe('list')
  })

  it('localStorage が使えなくても落ちず、一列表示で開く（警告は残す）', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    expect(readViewMode(broken)).toBe('list')
    expect(() => writeViewMode('tree', broken)).not.toThrow()
    expect(warn).toHaveBeenCalledTimes(2)
  })
})
