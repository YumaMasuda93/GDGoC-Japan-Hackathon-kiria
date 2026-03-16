package domain

import "context"

// EmbeddingClient はテキストと音声の埋め込み生成を担当するポートです。
type EmbeddingClient interface {
	EmbedText(ctx context.Context, text string) ([]float64, error)
	EmbedAudio(ctx context.Context, mimeType string, audioData []byte, title string) ([]float64, error)
	ModelName() string
}

// MusicGenerationClient はプロンプトから音楽クリップを生成するポートです。
type MusicGenerationClient interface {
	GenerateMusic(ctx context.Context, req MusicGenerationRequest) (MusicGenerationOutput, error)
	ModelName() string
}

// AudioRepository は音声メタデータと埋め込みベクトルを永続化するポートです。
type AudioRepository interface {
	InsertAudioRecord(ctx context.Context, originalFilename, storedFilename, mimeType string, fileSizeBytes int64, embeddingModel string, embedding []float64) (AudioRecord, error)
	GetAudioRecord(ctx context.Context, id int64) (AudioRecord, error)
	ListAudioRecords(ctx context.Context) ([]StoredAudioEmbedding, error)
	Close() error
}

// AudioStorage は後で再生する音声ファイル本体を保存するポートです。
type AudioStorage interface {
	SaveAudio(ctx context.Context, storedFilename string, audioData []byte) error
	AudioPath(storedFilename string) string
}

// StoredAudioEmbedding は音声メタデータと保存済みベクトルを束ねた値です。
type StoredAudioEmbedding struct {
	Record    AudioRecord
	Embedding []float64
}
