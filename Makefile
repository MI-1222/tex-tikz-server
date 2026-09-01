.PHONY: all gen build test test-integration clean run vet help

# デフォルトターゲット。
all: gen build

# OpenAPI スキーマからの Go コード自動生成。
gen:
	go generate ./...

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

# 静的解析・整合性チェック。
vet:
	go vet ./...

# ローカルサーバー起動。
run:
	go run ./cmd/server

# 一時ファイルやビルド成果物のクリーンアップ。
clean:
	rm -rf bin/ tmp/

# コマンド一覧のヘルプ表示。
help:
	@echo "利用可能な make コマンド:"
	@echo "\tmake gen              - OpenAPI スキーマから Go コードを自動生成"
	@echo "\tmake build            - サーバーバイナリ (bin/server) をビルド"
	@echo "\tmake test             - 単体テストを実行"
	@echo "\tmake test-integration - 結合テストを実行"
	@echo "\tmake vet              - go vet による静的解析を実行"
	@echo "\tmake run              - ローカルでサーバーを起動"
	@echo "\tmake clean            - ビルド成果物および一時ファイルを削除"
