import { describe, expect, it } from 'vitest'
import { assertCertifications, loadData } from './load'
import type { Roadmap } from './types'

// data/ の読み込みを確かめる。
// 「何を保証しているか」：
// - 資格が指す分野のキーがロードマップに無ければ、例外にする（打ち間違いで資格が黙って消えない）
// - 実物の data/（certifications.json を含む）を読み込める

const roadmap: Roadmap = {
  schemaVersion: 1,
  name: 'テスト',
  levels: [],
  domains: [{ key: 'go', name: 'Go', items: [] }],
}

describe('資格の検査', () => {
  it('ロードマップにある分野のキーなら通る', () => {
    expect(() => assertCertifications([{ domainKey: 'go', name: 'A' }], roadmap)).not.toThrow()
  })

  it('ロードマップに無い分野のキーがあれば、そのキーを挙げて例外にする', () => {
    expect(() =>
      assertCertifications(
        [
          { domainKey: 'go', name: 'A' },
          { domainKey: 'java', name: 'B' },
        ],
        roadmap,
      ),
    ).toThrow(/java/)
  })

  it('実物の data/ を読み込める（資格の分野のキーがロードマップにある）', () => {
    const data = loadData()
    expect(data.certifications?.length).toBeGreaterThan(0)
  })
})
