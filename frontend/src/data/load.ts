import roadmapJson from '@data/roadmap.json'
import stateJson from '@data/state.json'
import settingsJson from '@data/settings.json'
import certificationsJson from '@data/certifications.json'
import type { AppData, Certification, Roadmap, Settings, State } from './types'

// data/*.json をビルド時に取り込む。サーバ通信は無い（SPEC.md §8.2）。
// JSON の形は CLI の verify が CI で検査する前提だが、画面が壊れた JSON を黙って表示しないよう
// schemaVersion だけはここでも見る（握りつぶし禁止）。

/** schemaVersion が想定と違えば例外にする。想定外の JSON で画面が静かに崩れるのを防ぐ */
function assertSchema(name: string, actual: unknown, expected: number): void {
  if (actual !== expected) {
    throw new Error(`${name} の schemaVersion が ${String(actual)}（想定 ${expected}）`)
  }
}

/** リポジトリの URL。settings.json の site.repoUrl が無いときの既定（このリポジトリ自身） */
export const DEFAULT_REPO_URL = 'https://github.com/n-yoshida-dev/skill-matrix'

/**
 * 資格が指す分野のキーが、ロードマップにあるかを確かめる。無ければ例外にする
 * （打ち間違いで資格が黙って消えるのを防ぐ。CLI の verify は certifications.json を読まないので、ここで見る）
 */
export function assertCertifications(certs: Certification[], roadmap: Roadmap): void {
  const keys = new Set(roadmap.domains.map((d) => d.key))
  const unknown = certs.filter((c) => !keys.has(c.domainKey))
  if (unknown.length > 0) {
    throw new Error(
      `data/certifications.json に、ロードマップに無い分野のキーがある：${unknown.map((c) => c.domainKey).join('、')}`,
    )
  }
}

/** ロードマップ・理解度・設定・資格を読み込んで、画面に渡す形にまとめる */
export function loadData(): AppData {
  assertSchema('data/roadmap.json', roadmapJson.schemaVersion, 1)
  assertSchema('data/state.json', stateJson.schemaVersion, 2)
  assertSchema('data/certifications.json', certificationsJson.schemaVersion, 1)
  const roadmap = roadmapJson as Roadmap
  const certifications = certificationsJson.certifications as Certification[]
  assertCertifications(certifications, roadmap)
  return {
    roadmap,
    state: stateJson as State,
    settings: settingsJson as Settings,
    certifications,
  }
}
