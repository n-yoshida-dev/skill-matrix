# このリポジトリを実際のアプリにする手順

`app-template` から作った直後の状態。**この作業は `~/workspace/apps/` 直下のセッションで行う。**
すべて終わったら **このファイルを削除してコミットする。**

## 1. 雛形を埋める

`{{ }}` で囲まれた箇所がプレースホルダ。全ファイルを走査して残りがないことを確認する。

```bash
grep -rn '{{' --exclude-dir=.git .
```

- `CLAUDE.md` — アプリ名・一行説明・固有の制約
- `README.md` — アプリ名・概要・セットアップ手順
- `PLAN.md` — 何を作るか・なぜ作るか（走り書きでよい）
- `HANDOFF.md` — 「現在地」だけ埋める。残りは `/handoff` が育てる

`SPEC.md` `TODO.md` `KNOWLEDGE.md` は空のまま始めてよい。

## 2. フックに実行権限を付ける

git は実行ビットを保存するが、テンプレート経由だと落ちていることがある。

```bash
chmod +x .claude/hooks/*.sh
ls -l .claude/hooks/
```

## 3. プロジェクトを初期化する

必要なほうだけ。CI は `frontend/package.json` と `backend/go.mod` の有無を見て自動でジョブを出し分けるので、
片方だけでも CI は緑になる。

```bash
# フロントエンド
npm create vite@latest frontend -- --template react-ts

# バックエンド
mkdir backend && cd backend && go mod init github.com/n-yoshida-dev/{アプリ名}
```

フロントエンドを作ったら `package.json` に `typecheck` / `lint` / `format:check` / `test` / `build` の
スクリプトを揃える。CI がこの5つを呼ぶ。

## 4. 開き直す

```bash
code ~/workspace/apps/{アプリ名}
```

**以降の設計・実装はこの新しいウィンドウで行う。** `~/workspace/apps/` 直下のセッションは閉じてよい。

## 5. このファイルを消す

```bash
rm SETUP.md
git add -A && git commit -m "chore: テンプレートから {アプリ名} を初期化"
```
