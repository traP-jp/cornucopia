# Cornucopia

<img width="512" height="512" alt="24_20251214231141" src="https://github.com/user-attachments/assets/9ee12975-fd79-48b2-9449-280335569e77" />

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

プロトコルバッファ定義（[upstream](https://raw.githubusercontent.com/traP-jp/plutus/main/specs/protobuf/cornucopia.proto)）からコードを生成するには以下を実行します。

### 生成方法

Goスクリプト（`cmd/genproto`）を使用してコード生成を行います。
ローカルに `protoc` がない場合は Docker を使用します。

```bash
go run cmd/genproto/main.go
```
