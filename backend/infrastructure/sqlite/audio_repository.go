package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kiria/backend/domain"

	_ "modernc.org/sqlite"
)

// AudioRepository は音声メタデータと埋め込みを SQLite に保存します。
type AudioRepository struct {
	db *sql.DB
}

// NewAudioRepository は SQLite を開き、必要なテーブルを準備します。
func NewAudioRepository(dbPath string) (*AudioRepository, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	repo := &AudioRepository{db: db}
	if err := repo.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	return repo, nil
}

// Close はデータベース接続を閉じます。
func (r *AudioRepository) Close() error {
	return r.db.Close()
}

// InsertAudioRecord は音声メタデータと埋め込みベクトルを保存します。
func (r *AudioRepository) InsertAudioRecord(ctx context.Context, originalFilename, storedFilename, mimeType string, fileSizeBytes int64, embeddingModel string, embedding []float64) (domain.AudioRecord, error) {
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return domain.AudioRecord{}, fmt.Errorf("marshal embedding: %w", err)
	}

	createdAt := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO audio_embeddings (
			original_filename,
			stored_filename,
			mime_type,
			file_size_bytes,
			embedding_model,
			embedding_json,
			embedding_dimensions,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, originalFilename, storedFilename, mimeType, fileSizeBytes, embeddingModel, string(embeddingJSON), len(embedding), createdAt.Format(time.RFC3339))
	if err != nil {
		return domain.AudioRecord{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return domain.AudioRecord{}, err
	}

	return domain.AudioRecord{
		ID:               id,
		OriginalFilename: originalFilename,
		StoredFilename:   storedFilename,
		MIMEType:         mimeType,
		FileSizeBytes:    fileSizeBytes,
		EmbeddingModel:   embeddingModel,
		EmbeddingDims:    len(embedding),
		CreatedAt:        createdAt,
	}, nil
}

// GetAudioRecord はIDから音声レコードを1件取得します。
func (r *AudioRepository) GetAudioRecord(ctx context.Context, id int64) (domain.AudioRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			original_filename,
			stored_filename,
			mime_type,
			file_size_bytes,
			embedding_model,
			embedding_dimensions,
			created_at
		FROM audio_embeddings
		WHERE id = ?
	`, id)

	var record domain.AudioRecord
	var createdAt string
	if err := row.Scan(
		&record.ID,
		&record.OriginalFilename,
		&record.StoredFilename,
		&record.MIMEType,
		&record.FileSizeBytes,
		&record.EmbeddingModel,
		&record.EmbeddingDims,
		&createdAt,
	); err != nil {
		return domain.AudioRecord{}, err
	}

	record.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return record, nil
}

// ListAudioRecords は保存済み音声と埋め込みをすべて読み出します。
func (r *AudioRepository) ListAudioRecords(ctx context.Context) ([]domain.StoredAudioEmbedding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id,
			original_filename,
			stored_filename,
			mime_type,
			file_size_bytes,
			embedding_model,
			embedding_json,
			embedding_dimensions,
			created_at
		FROM audio_embeddings
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]domain.StoredAudioEmbedding, 0)
	for rows.Next() {
		var record domain.AudioRecord
		var embeddingJSON string
		var createdAt string
		if err := rows.Scan(
			&record.ID,
			&record.OriginalFilename,
			&record.StoredFilename,
			&record.MIMEType,
			&record.FileSizeBytes,
			&record.EmbeddingModel,
			&embeddingJSON,
			&record.EmbeddingDims,
			&createdAt,
		); err != nil {
			return nil, err
		}

		var embedding []float64
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			return nil, fmt.Errorf("unmarshal embedding for id %d: %w", record.ID, err)
		}

		record.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, domain.StoredAudioEmbedding{
			Record:    record,
			Embedding: embedding,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *AudioRepository) migrate() error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS audio_embeddings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_filename TEXT NOT NULL,
		stored_filename TEXT NOT NULL UNIQUE,
		mime_type TEXT NOT NULL,
		file_size_bytes INTEGER NOT NULL,
		embedding_model TEXT NOT NULL,
		embedding_json TEXT NOT NULL,
		embedding_dimensions INTEGER NOT NULL,
		created_at TEXT NOT NULL
	);
	`

	_, err := r.db.Exec(stmt)
	return err
}
