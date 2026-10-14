import { describe, expect, it } from 'vitest'
import { RADAR, easeOut, labelLines, labelPlace, ringPoints, toPoints, valuePoints } from './radar'

// 能力値チャートの形の計算を確かめる。
// 「何を保証しているか」：
// - 最初の分野は真上、時計回りに等間隔に並ぶ
// - 段の値は中心から外へ L0〜L5 の比で置き、5 を超える値は 5 に止める
// - 大きさの倍率 0 で全部が中心に集まり、1 で本来の形になる（広がる動きの始めと終わり）
// - 分野名は、上下の軸なら中央寄せ、右なら左寄せ、左なら右寄せ。長い名前だけ区切りで 2 行に折り、行の数に合わせて上下をずらす
// - 速さの変わり方は 0 で 0、1 で 1、途中は直線より先に進む（だんだん遅くなる）

const { cx, cy, radius } = RADAR

describe('多角形の頂点', () => {
  it('最初の分野は真上、2 つ目は右（4 分野なら 90 度ずつ時計回り）', () => {
    const [top, right, bottom, left] = valuePoints([5, 5, 5, 5], 1)
    expect(top[0]).toBeCloseTo(cx)
    expect(top[1]).toBeCloseTo(cy - radius)
    expect(right[0]).toBeCloseTo(cx + radius)
    expect(bottom[1]).toBeCloseTo(cy + radius)
    expect(left[0]).toBeCloseTo(cx - radius)
  })

  it('段の値は中心からの距離の比になり、5 を超える値は 5 に止める', () => {
    const [p] = valuePoints([3, 0, 0], 1)
    expect(cy - p[1]).toBeCloseTo((radius * 3) / 5)
    const [q] = valuePoints([9, 0, 0], 1)
    expect(cy - q[1]).toBeCloseTo(radius)
  })

  it('倍率 0 なら全部が中心に集まり、倍率 0.5 なら半分の大きさ', () => {
    for (const [x, y] of valuePoints([1, 3, 5], 0)) {
      expect(x).toBeCloseTo(cx)
      expect(y).toBeCloseTo(cy)
    }
    const [half] = valuePoints([4, 0, 0], 0.5)
    expect(cy - half[1]).toBeCloseTo((radius * 2) / 5)
  })

  it('目盛りの輪は軸の数と同じ角で、L5 の輪の半径は図の半径', () => {
    const ring = ringPoints(8, 5)
    expect(ring).toHaveLength(8)
    expect(cy - ring[0][1]).toBeCloseTo(radius)
  })

  it('SVG の points は小数 1 桁の「x,y」を空白でつなぐ', () => {
    expect(
      toPoints([
        [1, 2.25],
        [3.04, 4],
      ]),
    ).toBe('1.0,2.3 3.0,4.0')
  })
})

describe('分野名の置き場所', () => {
  it('上下の軸は中央寄せで、真上は少し上・真下は少し下へずらす', () => {
    const top = labelPlace(0, 4)
    const bottom = labelPlace(2, 4)
    expect(top.anchor).toBe('middle')
    expect(bottom.anchor).toBe('middle')
    expect(top.y).toBeLessThan(cy - radius - 22)
    expect(bottom.y).toBeGreaterThan(cy + radius + 22)
  })

  it('長い分野名だけ、「 / 」の後・「・」の後・「（」の前で 2 行に折る', () => {
    expect(labelLines('Go')).toEqual(['Go'])
    expect(labelLines('データベース設計')).toEqual(['データベース設計'])
    expect(labelLines('BaaS・認証')).toEqual(['BaaS・認証'])
    expect(labelLines('React / TypeScript')).toEqual(['React /', 'TypeScript'])
    expect(labelLines('CI/CD・GitHub 運用')).toEqual(['CI/CD・', 'GitHub 運用'])
    expect(labelLines('課金（Stripe）')).toEqual(['課金', '（Stripe）'])
  })

  it('2 行の名前は、真上なら上へ 1 行分ずらし、左右なら半行分上げ、真下ならそのまま', () => {
    const top1 = labelPlace(0, 4, 1)
    const top2 = labelPlace(0, 4, 2)
    expect(top1.y - top2.y).toBeCloseTo(top2.lineHeight)
    const side1 = labelPlace(1, 4, 1)
    const side2 = labelPlace(1, 4, 2)
    expect(side1.y - side2.y).toBeCloseTo(side2.lineHeight / 2)
    expect(labelPlace(2, 4, 2).y).toBeCloseTo(labelPlace(2, 4, 1).y)
  })

  it('右の軸は左寄せ、左の軸は右寄せ', () => {
    expect(labelPlace(1, 4).anchor).toBe('start')
    expect(labelPlace(3, 4).anchor).toBe('end')
  })
})

describe('動きの速さ', () => {
  it('0 で 0、1 で 1、途中は直線より先に進む。範囲の外は端に止める', () => {
    expect(easeOut(0)).toBe(0)
    expect(easeOut(1)).toBe(1)
    expect(easeOut(0.5)).toBeGreaterThan(0.5)
    expect(easeOut(2)).toBe(1)
    expect(easeOut(-1)).toBe(0)
  })
})
