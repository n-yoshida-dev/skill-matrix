import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import { buildPath } from './path'
import { TreeView } from './TreeView'

// スキルツリー表示の描き分け（SPEC.md §7.2 の 4 状態とゴーストノード）を確かめる。
// 配置の計算そのものは layout.test.ts が確かめる。ここは「どの節がどう見えるか」だけ。
// 「何を保証しているか」：
// - 未解放（前提にレベル 0 がある）は灰色で鍵マーク、確認済みでない直接の前提の名前が「必要：」で出る
// - 前提が確認済みの線だけに色が付く（行き先の状態は見ない）
// - 前提が 2 つ以上の項目には合流点があり、「n つとも必要 x/n」が出る。そろったときだけ下りる線に色が付く
// - 解放済み・未着手は枠を強調し、今ここの節に「今ここ」の札
// - 習得済みは塗り = verifiedLevel。要再確認は枠線、上の段にも根拠があれば角の三角
// - 深掘り候補は習得済みの節にだけ札が付く
// - 他分野の前提はゴーストノードで、押すとその分野の学習パスへ移るリンク
// - 節を押すと項目詳細へ移る

const item = (over: Partial<ItemState> & { itemKey: string }): ItemState => ({
  verifiedLevel: 0,
  evidencedLevels: [],
  preState: 'none',
  needsReview: false,
  lastEvidenceAt: null,
  events: [],
  ...over,
})

const today = new Date(2026, 8, 29)

const data: AppData = {
  roadmap: {
    schemaVersion: 1,
    name: 'テスト用',
    levels: [],
    domains: [
      {
        key: 'go',
        name: 'Go',
        items: [
          { key: 'go-01', name: '基本構文', outcome: '型と値の流れを追える' },
          { key: 'go-02', name: 'メソッド', dependsOn: ['go-01'] },
          { key: 'go-03', name: 'エラー', dependsOn: ['go-02'] },
          { key: 'go-04', name: 'HTTP', dependsOn: ['go-01', 'react-01'] },
          { key: 'go-05', name: '古い項目' },
          // 前提（go-03）は未達だが、この項目自体は習得済み
          { key: 'go-06', name: 'テスト', dependsOn: ['go-03'] },
        ],
      },
      { key: 'react', name: 'React', items: [{ key: 'react-01', name: 'fetch' }] },
    ],
  },
  state: {
    schemaVersion: 2,
    items: [
      item({
        itemKey: 'go-01',
        verifiedLevel: 2,
        evidencedLevels: [1, 2, 3],
        lastEvidenceAt: '2026-09-20',
      }),
      // 最終根拠が 150 日前（古い）
      item({
        itemKey: 'go-05',
        verifiedLevel: 1,
        evidencedLevels: [1],
        lastEvidenceAt: '2026-05-02',
      }),
      item({
        itemKey: 'go-06',
        verifiedLevel: 2,
        evidencedLevels: [1, 2],
        lastEvidenceAt: '2026-09-25',
      }),
    ],
    deferred: [],
    rejected: [],
  },
  settings: {},
}

function renderTree(deepen: string[] = []) {
  const states = new Map(data.state.items.map((s) => [s.itemKey, s]))
  const path = buildPath(data.roadmap.domains[0], states)
  return render(
    <MemoryRouter>
      <TreeView data={data} path={path} deepen={new Set(deepen)} today={today} />
    </MemoryRouter>,
  )
}

/** 項目キーで節（リンク）を探す */
const node = (key: string) => screen.getByText(key).closest('a') as HTMLElement

describe('スキルツリー表示', () => {
  it('未解放は灰色で鍵マーク、確認済みでない直接の前提の名前が「必要：」で出る', () => {
    renderTree()
    const n = node('go-03')
    expect(n).toHaveClass('locked')
    expect(n.querySelector('svg.lock')).not.toBeNull()
    expect(n).toHaveAccessibleName(/未解放/)
    expect(n).toHaveTextContent('必要：メソッド')
    // go-04 の前提は go-01（確認済み）と react-01（他分野・未確認）。出るのは react-01 だけ
    expect(node('go-04')).toHaveTextContent('必要：fetch')
    expect(node('go-04')).not.toHaveTextContent('基本構文')
  })

  it('前提が確認済みの線だけに色が付く。行き先が未解放でも付く', () => {
    const { container } = renderTree()
    // 線は 5 本：go-01→go-02、go-02→go-03、go-01→go-04、react-01→go-04、go-03→go-06
    expect(container.querySelectorAll('path.edge')).toHaveLength(5)
    // 色が付くのは前提が go-01（レベル 2）の 2 本だけ。go-01→go-04 の行き先 go-04 は未解放
    expect(container.querySelectorAll('path.edge.lit')).toHaveLength(2)
  })

  it('前提が未達でも習得済みの節は未解放にせず、「必要：」も出さない', () => {
    renderTree()
    expect(node('go-06')).toHaveClass('done', 'l2')
    expect(node('go-06').querySelector('svg.lock')).toBeNull()
    expect(node('go-06')).not.toHaveTextContent('必要：')
  })

  it('到達状態が空の節は「到達状態 未記入」と分かる', () => {
    renderTree()
    expect(node('go-02')).toHaveTextContent('到達状態 未記入')
    expect(node('go-01')).not.toHaveTextContent('未記入')
  })

  it('解放済み・未着手は枠を強調し、今ここの節に札が付く', () => {
    renderTree()
    const n = node('go-02')
    expect(n).toHaveClass('open', 'current')
    expect(n).toHaveTextContent('今ここ')
  })

  it('習得済みは塗り = verifiedLevel、上の段にも根拠があれば角の三角', () => {
    renderTree()
    expect(node('go-01')).toHaveClass('done', 'l2', 'up', 'up-l3')
  })

  it('要再確認（根拠が古い）は枠線', () => {
    renderTree()
    expect(node('go-05')).toHaveClass('done', 'l1', 'attn')
    expect(node('go-01')).not.toHaveClass('attn')
  })

  it('深掘り候補は習得済みの節にだけ札が付く', () => {
    renderTree(['go-01', 'go-02'])
    expect(node('go-01')).toHaveTextContent('深掘り候補')
    expect(node('go-02')).not.toHaveTextContent('深掘り候補')
  })

  it('前提が 2 つ以上の項目（go-04）の上に合流点を描き、「2 つとも必要 1/2」と出す', () => {
    const { container } = renderTree()
    // go-04 の前提は go-01（レベル 2）と react-01（他分野・レベル 0）
    const junctions = container.querySelectorAll('g.junction')
    expect(junctions).toHaveLength(1)
    expect(junctions[0]).toHaveTextContent('2 つとも必要 1/2')
    expect(junctions[0].querySelector('circle')).not.toBeNull()
    // 前提がそろっていないので、合流点から下りる線には色が付かない
    expect(junctions[0]).not.toHaveClass('open')
  })

  it('他分野の前提はゴーストノードで、押すとその分野の学習パスへ移る', () => {
    renderTree()
    const ghost = screen.getByText('他分野：React').closest('a') as HTMLElement
    expect(ghost).toHaveClass('ghost')
    expect(ghost).toHaveAttribute('href', '/plan/path/react')
    expect(ghost).toHaveTextContent('fetch')
  })

  it('節を押すと項目詳細へ移る', () => {
    renderTree()
    expect(node('go-01')).toHaveAttribute('href', '/plan/item/go-01')
  })
})
