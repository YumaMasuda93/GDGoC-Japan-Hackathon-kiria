package domain

import "time"

// AudioRecord はリポジトリに保存された音声データ1件を表します。
type AudioRecord struct {
	ID               int64     `json:"id"`
	OriginalFilename string    `json:"originalFilename"`
	StoredFilename   string    `json:"storedFilename"`
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

// SearchResponse は類似度順に並んだ音声検索結果を表します。
type SearchResponse struct {
	Query   string        `json:"query"`
	Results []AudioRecord `json:"results"`
}

// HealthResponse はヘルスチェックAPIのレスポンスです。
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Model     string `json:"model"`
}

// IndexResult は事前埋め込み1件の結果を表します。
type IndexResult struct {
	ID                  int64
	OriginalFilename    string
	StoredFilename      string
	MIMEType            string
	FileSizeBytes       int64
	EmbeddingDimensions int
}
