# syntax=docker/dockerfile:1
#
# backend（HTTP サーバ）のイメージ。ビルド用と実行用の2段に分けている（マルチステージビルド）。
# Go はビルドすると単一の実行ファイルになるので、実行側にはコンパイラもソースも持ち込まない。
#
# ビルドコンテキストは backend/。秘密情報（.env）は .dockerignore で除外している。
# 設定はすべて実行時の環境変数で渡す（一覧は SPEC.md §8.3）。イメージには焼き込まない。

# go.mod の go ディレクティブと揃える。上げるときは両方を直す
ARG GO_VERSION=1.26

# --- ビルド用 -----------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# 依存の取得を先に済ませる。go.mod / go.sum が変わらない限りこの層はキャッシュが効き、
# ソースだけ直したときの再ビルドが速くなる
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0: C のライブラリに依存しない実行ファイルにする（実行側に libc が無いため）
# -trimpath / -ldflags="-s -w": ビルド環境のパスとデバッグ情報を落として小さくする
# サーバと判定ワーカーの2つを作る。中身の大半は共通なので、1つのイメージに両方を入れて
# 起動時にどちらを動かすか選ぶ（compose の entrypoint で切り替える）。
# イメージを分けるとビルドも更新も2回ぶんになり、依存のずれも起きやすい
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# --- 実行用 -------------------------------------------------------------------
# distroless はシェルもパッケージ管理も無い最小イメージ。攻撃される面を減らす。
# static は CA 証明書とタイムゾーン情報を含むので、GitHub への HTTPS 通信はそのまま動く。
# nonroot タグは root ではない一般ユーザー（uid 65532）で動く
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --from=build /out/worker /worker

# 既定の PORT（internal/config の defaultPort）。実際の待ち受けは環境変数 PORT で決まる
EXPOSE 8080
USER nonroot:nonroot
# 既定は HTTP サーバ。判定ワーカーとして動かすときは entrypoint を /worker に差し替える
ENTRYPOINT ["/server"]
