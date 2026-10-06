import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import { buildPath } from './path'
import { TreeView } from './TreeView'
import treeCss from './tree.css?raw'

// スキルツリー表示の描き分け（SPEC.md §7.2 の 3 状態とゴーストノード）を確かめる。
// 配置の計算そのものは layout.test.ts が確かめる。ここは「どの節がどう見えるか」だけ。
// 「何を保証しているか」：
// - 前提待ち（前提にレベル 0 がある）は灰色で鍵マーク、確認済みでない直接の前提の名前が「必要：」で出る
// - 前提が確認済みの線だけに色が付く（行き先の状態は見ない）
// - 前提が 2 つ以上の項目には合流点があり、「n つとも必要 x/n」が出る。そろったときだけ下りる線に色が付く
// - 挑戦できる（着手できてレベル 0）は「挑戦できる」と出る
// - 確認済み（レベル 1 以上）は●が verifiedLevel 個と「Lv n」。どの状態も「習得済み」と呼ばない
// - 要再確認の枠・上位の根拠の三角・レベルによる色の濃淡・点線・深掘り候補・今ここはツリーに出ない
// - 上位の根拠があって基礎が未確認の項目は「実装の根拠あり・基礎は未確認」と文字で出る
// - 他分野の前提はゴーストノードで、押すとその分野の学習パスへ移るリンク。状態は分野の項目と同じ見せ方
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
          // 実装の根拠（印 3）だけがあり、基礎（1・2）が未確認。前提は確認済みの他分野の項目
          { key: 'go-07', name: 'DB', dependsOn: ['react-02'] },
        ],
      },
      {
        key: 'react',
        name: 'React',
        items: [
          { key: 'react-01', name: 'fetch' },
          { key: 'react-02', name: 'hooks' },
        ],
      },
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
      item({ itemKey: 'go-07', evidencedLevels: [3], lastEvidenceAt: '2026-09-25' }),
      item({
        itemKey: 'react-02',
        verifiedLevel: 1,
        evidencedLevels: [1],
        lastEvidenceAt: '2026-09-25',
      }),
    ],
    deferred: [],
    rejected: [],
    retracted: [],
  },
  settings: {},
}

function renderTree() {
  const states = new Map(data.state.items.map((s) => [s.itemKey, s]))
  const path = buildPath(data.roadmap.domains[0], states)
  return render(
    <MemoryRouter>
      <TreeView data={data} path={path} />
    </MemoryRouter>,
  )
}

/** 項目キーで節（リンク）を探す */
const node = (key: string) => screen.getByText(key).closest('a') as HTMLElement

describe('スキルツリー表示', () => {
  it('前提待ちは灰色で鍵マーク、確認済みでない直接の前提の名前が「必要：」で出る', () => {
    renderTree()
    const n = node('go-03')
    expect(n).toHaveClass('locked')
    expect(n.querySelector('svg.lock')).not.toBeNull()
    expect(n).toHaveAccessibleName(/前提待ち/)
    expect(n).toHaveTextContent('必要：メソッド')
    // go-04 の前提は go-01（確認済み）と react-01（他分野・未確認）。出るのは react-01 だけ
    expect(node('go-04')).toHaveTextContent('必要：fetch')
    expect(node('go-04')).not.toHaveTextContent('基本構文')
  })

  it('前提が確認済みの線だけに色が付く。行き先が前提待ちでも付く', () => {
    const { container } = renderTree()
    // 線は 6 本：go-01→go-02、go-02→go-03、go-01→go-04、react-01→go-04、go-03→go-06、react-02→go-07
    expect(container.querySelectorAll('path.edge')).toHaveLength(6)
    // 色が付くのは前提が go-01（レベル 2）の 2 本と react-02（レベル 1）の 1 本。go-01→go-04 の行き先 go-04 は前提待ち
    expect(container.querySelectorAll('path.edge.lit')).toHaveLength(3)
  })

  it('挑戦できる項目（着手できてレベル 0）は「挑戦できる」と出る', () => {
    renderTree()
    const n = node('go-02')
    expect(n).toHaveClass('open')
    expect(n).toHaveTextContent('挑戦できる')
    expect(n).toHaveAccessibleName(/挑戦できる/)
  })

  it('確認済みは●が verifiedLevel 個と「Lv n」。「習得済み」とは呼ばない', () => {
    renderTree()
    expect(node('go-01')).toHaveClass('done')
    expect(node('go-01')).toHaveTextContent('●●○○○Lv2')
    expect(node('go-01')).toHaveAccessibleName(/確認済み・レベル 2/)
    expect(node('go-05')).toHaveTextContent('●○○○○Lv1')
    expect(document.body).not.toHaveTextContent('習得済み')
  })

  it('前提が未達でも確認済みの節は前提待ちにせず、「必要：」も出さない', () => {
    renderTree()
    expect(node('go-06')).toHaveClass('done')
    expect(node('go-06').querySelector('svg.lock')).toBeNull()
    expect(node('go-06')).not.toHaveTextContent('必要：')
  })

  it('要再確認の枠・上位の根拠の三角・色の濃淡・点線・深掘り候補・今ここはツリーに出ない', () => {
    const { container } = renderTree()
    // go-01 は上位の根拠あり（印 3）、go-05 は根拠が古い（要再確認）、go-02 は一列表示なら今ここ
    for (const cls of ['attn', 'up', 'current', 'l1', 'l2', 'l3', 'l4', 'l5']) {
      expect(container.querySelector(`.tnode.${cls}`)).toBeNull()
    }
    expect(container.querySelector('.edge.ghost')).toBeNull()
    // jsdom はスタイルを計算しないので、点線はツリーのスタイルの中身で確かめる
    expect(treeCss).not.toMatch(/dashed|dasharray/)
    expect(container).not.toHaveTextContent('深掘り候補')
    expect(container).not.toHaveTextContent('今ここ')
  })

  it('上位の根拠があって基礎が未確認の項目は「実装の根拠あり・基礎は未確認」と文字で出る', () => {
    renderTree()
    expect(node('go-07')).toHaveClass('open')
    expect(node('go-07')).toHaveTextContent('実装の根拠あり・基礎は未確認')
    // go-01 は印 3 があるが 1・2 も確認済みなので出さない
    expect(node('go-01')).not.toHaveTextContent('基礎は未確認')
  })

  it('到達状態が空の節は「到達状態 未記入」と分かる', () => {
    renderTree()
    expect(node('go-02')).toHaveTextContent('到達状態 未記入')
    expect(node('go-01')).not.toHaveTextContent('未記入')
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

  it('他分野の前提はゴーストノードで、押すとその分野の学習パスへ移る。状態は分野の項目と同じ見せ方', () => {
    renderTree()
    const ghost = (name: string) => screen.getByText(name).closest('a.ghost') as HTMLElement
    expect(ghost('fetch')).toHaveAttribute('href', '/plan/path/react')
    // react-01 は前提が無くレベル 0 → 挑戦できる。react-02 はレベル 1 → 確認済み
    expect(ghost('fetch')).toHaveClass('open')
    expect(ghost('fetch')).toHaveTextContent('挑戦できる')
    expect(ghost('hooks')).toHaveClass('done')
    expect(ghost('hooks')).toHaveTextContent('●○○○○Lv1')
  })

  it('節を押すと項目詳細へ移る', () => {
    renderTree()
    expect(node('go-01')).toHaveAttribute('href', '/plan/item/go-01')
  })
})
