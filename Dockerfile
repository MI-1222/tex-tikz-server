# ==============================================================================
# Stage 1: Go アプリケーションの静的ビルド (Builder)
# ==============================================================================
FROM golang:alpine AS builder

WORKDIR /src

# 依存モジュール定義のコピーとダウンロード
COPY go.mod go.sum ./
RUN go mod download

# ソースコードのコピー
COPY . .

# CGO を無効化した静的バイナリのビルド (デバッグ情報の削除による軽量化)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/server \
    ./cmd/server

# ==============================================================================
# Stage 2: TeX 実行環境および本番ランタイム (Runner)
# ==============================================================================
FROM debian:bookworm-slim AS runner

# 非対話モードでのインストール設定
ENV DEBIAN_FRONTEND=noninteractive

# 最小限の TeX Live / 日本語環境 / dvisvgm のインストール
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    texlive-lang-japanese \
    texlive-pictures \
    texlive-latex-recommended \
    texlive-science \
    texlive-latex-extra \
    dvisvgm \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
    && rm -rf /usr/share/doc/* /usr/share/man/* /usr/share/info/* /usr/share/locale/* \
    && rm -rf /usr/share/texlive/texmf-dist/doc \
    && rm -rf /usr/share/texlive/texmf-dist/source \
    && rm -rf /usr/share/texlive/texmf-dist/fonts/source

# 非 root ユーザーの作成 (セキュリティ対策)
RUN useradd -m -u 10001 -s /bin/sh appuser

WORKDIR /app

# ビルド済みバイナリのコピー
COPY --from=builder /app/server /app/server

# フォントマップ定義およびフォントディレクトリのコピー
COPY fonts/ /app/fonts/

# 所有権の設定
RUN chown -R appuser:appuser /app

# 非 root ユーザーに切り替え
USER appuser

# デフォルト環境変数の設定 (Cloud Run 向け)
ENV PORT=8080 \
    FONT_DIR=/app/fonts \
    CACHE_SIZE=2000 \
    EXEC_TIMEOUT_SEC=10

# 待受ポートの公開
EXPOSE 8080

# サーバーの起動
ENTRYPOINT ["/app/server"]
