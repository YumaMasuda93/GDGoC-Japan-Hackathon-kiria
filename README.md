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

音楽生成も使う場合は、加えて以下を設定してください。

- `GOOGLE_CLOUD_PROJECT`: Vertex AI を使う Google Cloud プロジェクト ID
- `VERTEX_AI_LOCATION`: 省略時は `us-central1`
- `VERTEX_LYRIA_MODEL`: 省略時は `lyria-002`
- `GOOGLE_APPLICATION_CREDENTIALS` または Application Default Credentials

`POST /api/music/generate` には次の JSON を送れます。

```json
{
  "prompt": "An energetic electronic dance track with a fast tempo and bright synths.",
  "negativePrompt": "vocals, slow tempo",
  "sampleCount": 2
}
```

レスポンスには保存済み WAV の URL (`/api/generated/...`) が返ります。埋め込み生成にも成功した場合は検索用の `/api/audio/{id}` も返ります。

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
