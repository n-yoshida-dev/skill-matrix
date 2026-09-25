import roadmapJson from '@data/roadmap.json'
import stateJson from '@data/state.json'
import settingsJson from '@data/settings.json'
import type { AppData, Roadmap, Settings, State } from './types'

// data/*.json をビルド時に取り込む。サーバ通信は無い（SPEC.md §8.2）。
// JSON の形は CLI の verify が CI で検査する前提だが、画面が壊れた JSON を黙って表示しないよう
// schemaVersion だけはここでも見る（握りつぶし禁止）。

/** schemaVersion が想定と違えば例外にする。想定外の JSON で画面が静かに崩れるのを防ぐ */
function assertSchema(name: string, actual: unknown, expected: number): void {
  if (actual !== expected) {
    throw new Error(`${name} の schemaVersion が ${String(actual)}（想定 ${expected}）`)
  }
}

/** ロードマップ・理解度・設定を読み込んで、画面に渡す形にまとめる */
export function loadData(): AppData {
  assertSchema('data/roadmap.json', roadmapJson.schemaVersion, 1)
  assertSchema('data/state.json', stateJson.schemaVersion, 2)
  return {
    roadmap: roadmapJson as Roadmap,
    state: stateJson as State,
    settings: settingsJson as Settings,
  }
}
