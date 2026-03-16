package domain

import "time"

// AudioRecord describes one indexed audio item stored in the repository.
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

// SearchRequest is the request payload for text-to-audio retrieval.
type SearchRequest struct {
	Text  string `json:"text"`
	Limit int    `json:"limit"`
}

// SearchResponse contains ranked audio search results.
type SearchResponse struct {
	Query   string        `json:"query"`
	Results []AudioRecord `json:"results"`
}

// HealthResponse is returned by the health endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Model     string `json:"model"`
}

// IndexResult summarizes one offline indexing operation.
type IndexResult struct {
	ID                  int64
	OriginalFilename    string
	StoredFilename      string
	MIMEType            string
	FileSizeBytes       int64
	EmbeddingDimensions int
}
