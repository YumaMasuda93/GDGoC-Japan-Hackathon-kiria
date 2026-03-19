package domain

import "time"

// AudioRecord はリポジトリに保存された音声データ1件を表します。
type AudioRecord struct {
	ID               int64     `json:"id"`
	OriginalFilename string    `json:"originalFilename"`
	SourcePath       string    `json:"sourcePath"`
	MIMEType         string    `json:"mimeType"`
	FileSizeBytes    int64     `json:"fileSizeBytes"`
	EmbeddingModel   string    `json:"embeddingModel"`
	EmbeddingDims    int       `json:"embeddingDimensions"`
	CreatedAt        time.Time `json:"createdAt"`
	SimilarityScore  float64   `json:"similarityScore,omitempty"`
	DownloadURL      string    `json:"downloadUrl,omitempty"`
}

// SearchRequest はテキストから音声を検索するリクエストです。
type SearchRequest struct {
	Text  string `json:"text"`
	Limit int    `json:"limit"`
}

// SearchResultItem はフロントに返す検索結果1件を表します。
type SearchResultItem struct {
	ID               int64   `json:"id"`
	OriginalFilename string  `json:"originalFilename"`
	MIMEType         string  `json:"mimeType"`
	FileSizeBytes    int64   `json:"fileSizeBytes"`
	EmbeddingModel   string  `json:"embeddingModel"`
	EmbeddingDims    int     `json:"embeddingDimensions"`
	SimilarityScore  float64 `json:"similarityScore"`
	DownloadURL      string  `json:"downloadUrl"`
}

// SearchResponse は類似度順に並んだ音声検索結果を表します。
type SearchResponse struct {
	Query   string             `json:"query"`
	Results []SearchResultItem `json:"results"`
}

// MusicGenerationRequest はテキストから音楽を生成するリクエストです。
type MusicGenerationRequest struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negativePrompt,omitempty"`
	SampleCount    int    `json:"sampleCount,omitempty"`
	Seed           *int64 `json:"seed,omitempty"`
}

// GeneratedMusicClip は生成済み音楽クリップの保存結果です。
type GeneratedMusicClip struct {
	Filename        string `json:"filename"`
	MIMEType        string `json:"mimeType"`
	FileSizeBytes   int64  `json:"fileSizeBytes"`
	DownloadURL     string `json:"downloadUrl"`
	IndexedAudioID  *int64 `json:"indexedAudioId,omitempty"`
	IndexedAudioURL string `json:"indexedAudioUrl,omitempty"`
}

// MusicGenerationResponse は音楽生成APIのレスポンスです。
type MusicGenerationResponse struct {
	Prompt           string               `json:"prompt"`
	TranslatedPrompt string               `json:"translatedPrompt,omitempty"`
	NegativePrompt   string               `json:"negativePrompt,omitempty"`
	Model            string               `json:"model"`
	ModelDisplay     string               `json:"modelDisplayName,omitempty"`
	Clips            []GeneratedMusicClip `json:"clips"`
}

// GeneratedAudioSample は生成モデルが返した音声データです。
type GeneratedAudioSample struct {
	MIMEType  string
	AudioData []byte
}

// MusicGenerationOutput は生成モデルの生レスポンスを表します。
type MusicGenerationOutput struct {
	Model        string
	ModelDisplay string
	Clips        []GeneratedAudioSample
}

// HealthResponse はヘルスチェックAPIのレスポンスです。
type HealthResponse struct {
	Status     string `json:"status"`
	Timestamp  string `json:"timestamp"`
	Model      string `json:"model"`
	MusicModel string `json:"musicModel,omitempty"`
}

// IndexResult は事前埋め込み1件の結果を表します。
type IndexResult struct {
	ID                  int64
	OriginalFilename    string
	SourcePath          string
	MIMEType            string
	FileSizeBytes       int64
	EmbeddingDimensions int
}
