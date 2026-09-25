# frontend

skill-matrix の画面。React + TypeScript + Vite の静的サイトで、リポジトリ直下の `data/*.json` をビルド時に取り込む。
サーバ通信は無い。仕様は `../SPEC.md` §7・§8。

```bash
npm ci
npm run dev        # http://localhost:5173/ 。#/ が公開ビュー、#/plan が作業ビュー
npm run test       # Vitest
npm run build      # dist/ に静的サイトを出力（base は相対パス）
```

CI と同じ検査：`npm run format:check` → `npm run lint` → `npm run typecheck` → `npm run test` → `npm run build`。

## 構成

| 場所                   | 役割                                                                                                            |
| ---------------------- | --------------------------------------------------------------------------------------------------------------- |
| `src/data/`            | `data/*.json` の型と読み込み（`@data/...` で指す。別名は `vite.config.ts` と `tsconfig.app.json` の両方にある） |
| `src/features/public/` | 公開ビュー（SPEC §7.3）。集計は `summary.ts` の純粋関数                                                         |
| `src/features/plan/`   | 作業ビュー（SPEC §7 の 3 画面。実装中）                                                                         |
| `src/index.css`        | 色と文字の土台。各画面はここで付けた名前だけを使う                                                              |
