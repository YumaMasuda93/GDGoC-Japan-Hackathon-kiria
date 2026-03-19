# SQLite Schema

## 概要

`backend/data/kiria.db` は、音声埋め込み検索のための SQLite データベースです。  
現時点では `audio_embeddings` テーブルのみを持ち、音声メタデータと埋め込みベクトルを保存します。

スキーマ定義の元ファイルは [sqlite.sql](./sqlite.sql) です。  
実装上の migration は `backend/infrastructure/sqlite/audio_repository.go` にあります。

## テーブル一覧

### `audio_embeddings`

事前埋め込み済み音声のメタデータと埋め込みベクトルを保持します。

| カラム名 | 型 | 必須 | 説明 |
| --- | --- | --- | --- |
| `id` | `INTEGER` | Yes | 主キー。自動採番される音声ID |
| `original_filename` | `TEXT` | Yes | 元音声ファイル名 |
| `source_path` | `TEXT` | Yes | 音声ファイルの参照先。現状は主に `data/audio` 配下の相対パスを保存 |
| `mime_type` | `TEXT` | Yes | 音声ファイルの MIME type |
| `file_size_bytes` | `INTEGER` | Yes | 音声ファイルサイズ |
| `embedding_model` | `TEXT` | Yes | 埋め込み生成に使ったモデル名 |
| `embedding_json` | `TEXT` | Yes | 埋め込みベクトル本体。JSON 配列文字列で保存 |
| `embedding_dimensions` | `INTEGER` | Yes | 埋め込みベクトルの次元数 |
| `created_at` | `TEXT` | Yes | 登録時刻。UTC の RFC3339 文字列 |

## 運用上の注意

- `source_path` は主にアプリ管理下の相対パスです。`data/audio` 配下の保存ファイルを指す前提です。
- 旧データや環境差分がある場合でも、サーバーは `data/audio` や `seed` を候補にパス解決します。
- 埋め込みベクトルは `embedding_json` にそのまま保存しているため、件数が増えると検索コストは上がります。
- 検索は現時点で全件走査 + コサイン類似度計算です。大量データ向けのベクトルDBは未導入です。
- 旧実装で `stored_filename` カラムを持つ DB は、起動時 migration で `source_path` にリネームされます。

## 確認用 SQL

テーブル一覧:

```sql
.tables
```

スキーマ確認:

```sql
.schema audio_embeddings
```

登録済みデータ確認:

```sql
SELECT
  id,
  original_filename,
  source_path,
  mime_type,
  file_size_bytes,
  embedding_model,
  embedding_dimensions,
  created_at
FROM audio_embeddings;
```

埋め込みベクトルの先頭だけ確認:

```sql
SELECT
  id,
  substr(embedding_json, 1, 120) || '...'
FROM audio_embeddings;
```

## 実行例

```bash
cd backend
sqlite3 data/kiria.db
```
