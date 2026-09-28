import { describe, expect, it } from 'vitest'
import type { RoadmapDomain } from '../../data/types'
import { layoutTree } from './layout'

// スキルツリーの配置計算（layout.ts。SPEC.md §7.2）を確かめる。
// 「何を保証しているか」：
// - 段は依存の深さ（前提なし = 0 段、前提の最大段 + 1）。前提が上、派生が下
// - 同じ段は定義順で左から並べる
// - 他分野の前提はゴーストノードとして 0 段に 1 つだけ置き、そこから線を引く（ゴースト自身の依存は描かない）
// - 線は前提 → 派生の向きで、分野の中の依存と、ゴーストからの線を区別する
// - 循環参照があっても全項目を置いて落ちない
// - 段の数と、最も広い段の横の数を返す

// SPEC.md §7.2 の図の形：go-01 → go-02 → (go-03, go-04)、go-03 → go-05 → go-06。go-05 は react-03 にも依存
const goDomain: RoadmapDomain = {
  key: 'go',
  name: 'Go',
  items: [
    { key: 'go-01', name: '基本構文' },
    { key: 'go-02', name: 'メソッド', dependsOn: ['go-01'] },
    { key: 'go-03', name: 'エラー', dependsOn: ['go-02'] },
    { key: 'go-04', name: '並行処理', dependsOn: ['go-02'] },
    { key: 'go-05', name: 'net/http', dependsOn: ['go-03', 'react-03'] },
    { key: 'go-06', name: 'テスト', dependsOn: ['go-05'] },
  ],
}
const reactDomain: RoadmapDomain = {
  key: 'react',
  name: 'React',
  items: [{ key: 'react-03', name: 'fetch' }],
}
const all = [goDomain, reactDomain]

const pos = (l: ReturnType<typeof layoutTree>) =>
  Object.fromEntries(l.nodes.map((n) => [n.key, [n.row, n.col]]))

describe('layoutTree（スキルツリーの配置）', () => {
  it('段は依存の深さ、同じ段は定義順で左から', () => {
    const got = pos(layoutTree(goDomain, all))
    expect(got['go-01']).toEqual([0, 0])
    expect(got['go-02']).toEqual([1, 0])
    expect(got['go-03']).toEqual([2, 0])
    expect(got['go-04']).toEqual([2, 1])
    expect(got['go-05']).toEqual([3, 0])
    expect(got['go-06']).toEqual([4, 0])
  })

  it('他分野の前提はゴーストノードとして 0 段に置き、そこから線を引く', () => {
    const l = layoutTree(goDomain, all)
    const ghost = l.nodes.find((n) => n.key === 'react-03')
    expect(ghost).toEqual({ key: 'react-03', row: 0, col: 1, ghost: true, domainKey: 'react' })
    expect(l.edges).toContainEqual({ from: 'react-03', to: 'go-05', ghost: true })
    // ゴーストノード自身の依存は描かない（ゴーストへ入る線は無い）
    expect(l.edges.filter((e) => e.to === 'react-03')).toEqual([])
  })

  it('同じ他分野の前提を複数の項目が指しても、ゴーストノードは 1 つ', () => {
    const d: RoadmapDomain = {
      key: 'java',
      name: 'Java',
      items: [
        { key: 'j1', name: '', dependsOn: ['db-1'] },
        { key: 'j2', name: '', dependsOn: ['db-1'] },
      ],
    }
    const l = layoutTree(d, [d, { key: 'db', name: 'DB', items: [{ key: 'db-1', name: '' }] }])
    expect(l.nodes.filter((n) => n.ghost)).toHaveLength(1)
    expect(l.edges.filter((e) => e.from === 'db-1')).toHaveLength(2)
    // 前提（ゴースト）が 0 段なので、j1・j2 は 1 段
    expect(pos(l)['j1']).toEqual([1, 0])
    expect(pos(l)['j2']).toEqual([1, 1])
  })

  it('線は前提 → 派生の向きで、分野の中の依存はゴーストの線と区別する', () => {
    const l = layoutTree(goDomain, all)
    expect(l.edges).toContainEqual({ from: 'go-01', to: 'go-02', ghost: false })
    expect(l.edges).toContainEqual({ from: 'go-03', to: 'go-05', ghost: false })
    expect(l.edges).toHaveLength(6)
  })

  it('ロードマップのどこにも無い前提は置かない', () => {
    const d: RoadmapDomain = {
      key: 'x',
      name: '',
      items: [{ key: 'a', name: '', dependsOn: ['nowhere'] }],
    }
    const l = layoutTree(d, [d])
    expect(l.nodes.map((n) => n.key)).toEqual(['a'])
    expect(l.edges).toEqual([])
  })

  it('循環参照があっても全項目を置いて落ちない', () => {
    const d: RoadmapDomain = {
      key: 'x',
      name: '',
      items: [
        { key: 'a', name: '', dependsOn: ['b'] },
        { key: 'b', name: '', dependsOn: ['a'] },
        { key: 'c', name: '' },
      ],
    }
    const l = layoutTree(d, [d])
    expect(l.nodes.map((n) => n.key).sort()).toEqual(['a', 'b', 'c'])
  })

  it('段の数と、最も広い段の横の数を返す', () => {
    const l = layoutTree(goDomain, all)
    expect(l.rows).toBe(5)
    expect(l.cols).toBe(2)
  })

  it('項目が無い分野でも落ちない', () => {
    const l = layoutTree({ key: 'e', name: '', items: [] }, [])
    expect(l).toEqual({ nodes: [], edges: [], rows: 0, cols: 0 })
  })
})
