.PHONY: all gen gen-ts build test test-integration test-e2e test-e2e-docker clean run vet docker-build docker-buildx docker-size docker-run docker-stop test-container help

# デフォルトターゲット。
all: gen gen-ts build

# OpenAPI スキーマからの Go コード自動生成。
gen:
	go generate ./...

# OpenAPI スキーマからの TypeScript 型定義自動生成 (Obsidian プラグイン / クライアント向け)。
gen-ts:
	npx -y openapi-typescript api/openapi.yaml -o api/gen/types.ts
	@echo "TypeScript 型定義を生成しました: api/gen/types.ts"

# Go サーバーバイナリのビルド。
build:
	@mkdir -p bin
	go build -v -o bin/server ./cmd/server

# 単体テスト実行。
test:
	go test -v -race -cover ./...

# 結合テスト実行（integration タグ付きテスト）。
test-integration:
	go test -v -race -tags=integration ./...

# E2E シナリオテスト実行。
test-e2e:
	go test -v -race ./test/e2e/...

# Docker コンテナに対する E2E シナリオテスト実行。
test-e2e-docker: docker-run
	@echo "コンテナに対する E2E テストを実行します..."
	@sleep 2
	SERVER_URL=http://localhost:$(PORT) API_KEY=$(API_KEY) go test -v ./test/e2e/...
	@$(MAKE) docker-stop

# 静的解析・整合性チェック。
vet:
	go vet ./...

# ローカルサーバー起動。
run:
	go run ./cmd/server

# Docker 関連設定
IMAGE_NAME ?= tex-tikz-server
IMAGE_TAG ?= latest
PORT ?= 8080
API_KEY ?= test_dev_key_12345
CONTAINER_NAME ?= tex-tikz-server-local

# Docker イメージのビルド。
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "ビルド完了: $(IMAGE_NAME):$(IMAGE_TAG)"
	@docker images $(IMAGE_NAME):$(IMAGE_TAG) --format "Image Size: {{.Size}}"

# マルチプラットフォーム Docker イメージのビルド (linux/amd64, linux/arm64)。
docker-buildx:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Docker イメージサイズの確認。
docker-size:
	docker images $(IMAGE_NAME):$(IMAGE_TAG)

# Docker コンテナのローカル起動。
docker-run: docker-stop
	docker run -d --name $(CONTAINER_NAME) -p $(PORT):8080 -e API_KEY=$(API_KEY) $(IMAGE_NAME):$(IMAGE_TAG)
	@echo "コンテナを起動しました (http://localhost:$(PORT))"

# Docker コンテナの停止・削除。
docker-stop:
	@docker stop $(CONTAINER_NAME) >/dev/null 2>&1 || true
	@docker rm $(CONTAINER_NAME) >/dev/null 2>&1 || true

# コンテナ起動からテスト・停止までの自動検証。
test-container: docker-run
	@echo "コンテナの起動待機中..."
	@sleep 2
	@echo "1. ヘルスチェックテスト:"
	@curl -sf http://localhost:$(PORT)/health | grep '"status":"ok"' && echo " [PASS] /health 応答確認"
	@echo "2. TikZ レンダリングテスト (日本語含む):"
	@curl -sf -X POST http://localhost:$(PORT)/api/v1/render/tikz \
		-H "Content-Type: application/json" \
		-H "X-API-Key: $(API_KEY)" \
		-d '{"code":"\\begin{tikzpicture}\n\\draw (0,0) circle (1cm);\n\\node at (0,0) {コンテナテスト};\n\\end{tikzpicture}"}' | grep '"status":"success"' && echo " [PASS] TikZ レンダリング成功"
	@$(MAKE) docker-stop
	@echo "=== Phase 5 コンテナテスト完了: 全テスト正常終了 ==="

# 一時ファイルやビルド成果物のクリーンアップ。
clean:
	rm -rf bin/ tmp/

# コマンド一覧のヘルプ表示。
help:
	@echo "利用可能な make コマンド:"
	@echo "\tmake gen              - OpenAPI スキーマから Go コードを自動生成"
	@echo "\tmake gen-ts           - OpenAPI スキーマから TypeScript 型定義を自動生成"
	@echo "\tmake build            - サーバーバイナリ (bin/server) をビルド"
	@echo "\tmake test             - 単体テストを実行"
	@echo "\tmake test-integration - 結合テストを実行"
	@echo "\tmake test-e2e         - E2E シナリオテストを実行"
	@echo "\tmake test-e2e-docker  - Docker コンテナに対して E2E テストを実行"
	@echo "\tmake vet              - go vet による静的解析を実行"
	@echo "\tmake run              - ローカルでサーバーを起動"
	@echo "\tmake docker-build     - Docker イメージをビルドしてサイズを表示"
	@echo "\tmake docker-buildx    - マルチプラットフォーム (amd64/arm64) で Docker ビルド"
	@echo "\tmake docker-run       - Docker コンテナをバックグラウンドで起動"
	@echo "\tmake docker-stop      - 起動中の Docker コンテナを停止・削除"
	@echo "\tmake test-container   - コンテナ起動・ヘルスチェック・TikZ レンダリングの自動検証"
	@echo "\tmake clean            - ビルド成果物および一時ファイルを削除"
