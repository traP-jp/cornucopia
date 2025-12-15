# Cornucopia

## 必要要件

- Go 1.25.5
- Docker & Docker Compose

## セットアップと実行

1. **データベースの起動**
   ```bash
   docker-compose up -d
   ```

2. **アプリケーションの実行**
   ```bash
   go mod tidy
   go run cmd/cornucopia/main.go
   ```

## gRPCコード生成

プロトコルバッファ定義（`../../specs/protobuf`）に変更があった場合は以下を実行してください。

```bash
# ツールのインストール
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成実行
protoc -I ../../specs/protobuf --go_out=api/protobuf --go_opt=paths=source_relative --go-grpc_out=api/protobuf --go-grpc_opt=paths=source_relative ../../specs/protobuf/cornucopia.proto
```
