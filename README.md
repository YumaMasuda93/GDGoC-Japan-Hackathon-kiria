# Google Developer Groups on Campus Japan Hackathon - kiria

## Project Overview

Kiriaは、ユーザーとAIが共同で音楽制作を行うWebアプリです。

既存の音楽生成AIは、プロンプトを入力するだけで楽曲生成が完結するものが多く、ユーザーが制作プロセスに主体的に関与しづらいという課題があります。

そこで本システムでは、ユーザーが段階的に楽曲候補を選択しながら好みを反映し、最終的にAIが新しい楽曲を生成する体験を提供します。ユーザーの選択履歴を活用して音楽的な嗜好を推定し、単なる生成ではなく「AIとの共創」を目指しました。

ハッカソンチーム: kiria

## My Contributions

### 担当

* React + TypeScriptを用いたフロントエンド実装
* Gemini Embeddingを用いた音源ベクトル化機能の設計・実装

### 担当工程

* フロントエンド画面設計・実装
* 音源埋め込み生成パイプラインの設計・実装
* 動作検証およびデバッグ

## Challenges and Solutions

### Challenge

本システムでは、ユーザーが質問に回答しながら音楽制作を進めます。そのため、ユーザーの回答内容に応じて、好みに近い既存楽曲を提示する必要がありました。

しかし、ユーザーの入力はテキストである一方、検索対象は音楽データです。テキストと音楽を直接比較することはできず、どのようにしてユーザーの意図に近い楽曲を検索するかが課題でした。

### Solution

この課題を解決するため、Gemini Embeddingを利用しました。Gemini Embeddingはテキストと音楽を同一の埋め込み空間で表現できるため、ユーザーの回答文をベクトル化し、事前に埋め込み化した音源との類似度検索を行いました。

これにより、ユーザーの言葉から音楽的な嗜好を推定し、好みに近い楽曲を段階的に提示できるようになりました。その結果、単なる音楽生成ではなく、ユーザーが選択を重ねながらAIと共同で楽曲を制作する体験を実現しました。


## Technical Stack

### Frontend

* React
* TypeScript
* Vite

### Backend

* Go

### AI / Cloud

* Gemini Embedding
* Vertex AI
* Lyria

## Results

* 4名チームで1週間のハッカソン開発を完遂
* Gemini EmbeddingとLyriaを活用した音楽共創システムを実装
* ユーザーの選択履歴を反映した楽曲推薦および音楽生成機能を実現
* 企画立案から実装までを短期間で経験し、生成AIサービス開発の実践力を向上


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

### 既存ライブラリ 1000 曲を事前埋め込みする

事前に、[Google Drive の zip ファイル](https://drive.google.com/file/d/1Av_DQuH4GtZ7lWCQ2QuOuqEZahgzq7vg/view?usp=sharing)をダウンロードして展開し、展開後のフォルダを `backend/data` の中に配置してください。

`backend/data/all_datas_shuffle` の 1000 曲を別データに置き換えた場合は、`backend` 配下で次を実行します。

```bash
cd backend
RESET_DB=1 ./scripts/index_audio_library.sh
```

このスクリプトは以下を行います。

- 既定で `backend/data/all_datas_shuffle` を走査
- `RESET_DB=1` のとき `backend/data/kiria.db` を削除して作り直し
- 各曲の埋め込みを Gemini で生成
- 音源ファイルは `data/audio` へコピーせず、`data/all_datas_shuffle/...` をそのまま参照して DB 登録
- 途中で失敗した場合は `./scripts/index_audio_library.sh` を再実行すると未登録分だけ続行

個別に実行したい場合は、`indexer` へディレクトリを直接渡せます。

```bash
cd backend
go run ./cmd/indexer -reference -skip-existing ./data/all_datas_shuffle
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
