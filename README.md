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
go run cmd/server/main.go
```

ルートで `make dev` を実行すると、フロントエンドとバックエンドを同時に起動できます。

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
  "sampleCount": 4,
  "selectedAudioIds": [12, 18, 31, 42, 57]
}
```

`selectedAudioIds` を渡すと、サーバーは複数クリップを生成して各クリップを埋め込み化し、選択済み音源群との平均類似度が最も高い 1 本を返します。レスポンスには保存済み WAV の URL (`/api/generated/...`) が返ります。埋め込み生成にも成功した場合は検索用の `/api/audio/{id}` も返ります。

100 曲まとめて生成したい場合は、`backend` 配下で次を実行します。

```bash
cd backend
go run ./cmd/lyriabatch
```

この CLI は以下の流れで動きます。

- プロンプトは Gemini `gemini-2.5-flash` API で 10 件ずつ生成
- 音楽は Vertex AI の `lyria-002` で既定 10 並列で生成
- 音声ファイルは `backend/data/audio` に保存
- 生成した音声は Gemini `gemini-embedding-2-preview` で埋め込み化し、`backend/data/kiria.db` に保存
- 生成に使ったタイトル、プロンプト、保存先は `backend/data/lyria-batch-<timestamp>.json` に保存

必要なら件数、プロンプト生成バッチサイズ、並列数を変更できます。

```bash
cd backend
go run ./cmd/lyriabatch -count 100 -prompt-batch-size 10 -parallel 10
```

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
