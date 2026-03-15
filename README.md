# kiria

ハッカソンチーム: kiria

## 構成

```text
.
├── backend   # Go API サーバー
└── frontend  # React + TypeScript (Vite)
```

## セットアップ

### 1. フロントエンドの依存関係を入れる

```bash
cd frontend
npm install
```

### 2. バックエンドを起動する

```bash
cd backend
go run .
```

`http://localhost:8080/api/health` で API を確認できます。

### 3. フロントエンドを起動する

別ターミナルで:

```bash
cd frontend
npm run dev
```

`http://localhost:5173` を開くと、React から Go API に接続します。

## 現在の最小機能

- React + TypeScript の画面表示
- Go の `GET /api/health` API
- Vite の開発プロキシ経由で `/api` を Go に転送
