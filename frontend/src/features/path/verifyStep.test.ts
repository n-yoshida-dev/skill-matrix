import { describe, expect, it } from 'vitest'
import { verifyStep } from './verifyStep'

// このテストが保証すること
// - 末尾の「（L1）」から、段の名前と数字の見出しを作り、本文から段の表示を外す
// - 「（L1・L2）」のように 2 つあれば、上の段で言う
// - 末尾に段の表示が無ければ、見出しは「次のレベルに上がるには」で本文はそのまま

describe('verifyStep', () => {
  it('末尾の段から見出しを作り、本文から段の表示を外す', () => {
    expect(verifyStep('確認問題に答える（L1）')).toEqual({
      lead: '「基礎を確認した」（L1）になるには',
      text: '確認問題に答える',
    })
  })

  it('段が 2 つなら上の段で言う', () => {
    expect(verifyStep('何を防ぐかを答える（L1・L2）')).toEqual({
      lead: '「自分の言葉で説明できる」（L2）になるには',
      text: '何を防ぐかを答える',
    })
  })

  it('段の表示が無ければ本文はそのまま', () => {
    expect(verifyStep('ドリルに答える')).toEqual({
      lead: '次のレベルに上がるには',
      text: 'ドリルに答える',
    })
  })
})
