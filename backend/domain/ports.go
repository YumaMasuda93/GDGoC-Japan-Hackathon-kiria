package domain

import "context"

// EmbeddingClient creates embeddings for text and audio inputs.
type EmbeddingClient interface {
	EmbedText(ctx context.Context, text string) ([]float64, error)
	EmbedAudio(ctx context.Context, mimeType string, audioData []byte, title string) ([]float64, error)
	ModelName() string
}

// AudioRepository persists indexed audio metadata and embeddings.
type AudioRepository interface {
	InsertAudioRecord(ctx context.Context, originalFilename, storedFilename, mimeType string, fileSizeBytes int64, embeddingModel string, embedding []float64) (AudioRecord, error)
	GetAudioRecord(ctx context.Context, id int64) (AudioRecord, error)
	ListAudioRecords(ctx context.Context) ([]StoredAudioEmbedding, error)
	Close() error
}

// AudioStorage stores raw audio files for later retrieval.
type AudioStorage interface {
	SaveAudio(ctx context.Context, storedFilename string, audioData []byte) error
	AudioPath(storedFilename string) string
}

// StoredAudioEmbedding combines an audio record with its stored vector.
type StoredAudioEmbedding struct {
	Record    AudioRecord
	Embedding []float64
}
