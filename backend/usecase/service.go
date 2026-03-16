package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kiria/backend/domain"
)

// Service coordinates the application's indexing and retrieval flows.
type Service struct {
	embedding domain.EmbeddingClient
	repo      domain.AudioRepository
	storage   domain.AudioStorage
}

// NewService constructs the core application usecases.
func NewService(embedding domain.EmbeddingClient, repo domain.AudioRepository, storage domain.AudioStorage) *Service {
	return &Service{
		embedding: embedding,
		repo:      repo,
		storage:   storage,
	}
}

// ModelName returns the active embedding model used by the service.
func (s *Service) ModelName() string {
	return s.embedding.ModelName()
}

// Close releases repository resources.
func (s *Service) Close() error {
	return s.repo.Close()
}

// IndexAudioFile embeds a local audio file and stores both the file and vector.
func (s *Service) IndexAudioFile(ctx context.Context, sourcePath string) (domain.IndexResult, error) {
	audioBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return domain.IndexResult{}, fmt.Errorf("read audio file: %w", err)
	}
	if len(audioBytes) == 0 {
		return domain.IndexResult{}, errors.New("audio file is empty")
	}

	originalFilename := filepath.Base(sourcePath)
	mimeType := DetectMIMEType(originalFilename, audioBytes)

	vector, err := s.embedding.EmbedAudio(ctx, mimeType, audioBytes, originalFilename)
	if err != nil {
		return domain.IndexResult{}, fmt.Errorf("gemini audio embedding failed: %w", err)
	}

	storedFilename, err := BuildStoredFilename(originalFilename)
	if err != nil {
		return domain.IndexResult{}, fmt.Errorf("build stored filename: %w", err)
	}

	if err := s.storage.SaveAudio(ctx, storedFilename, audioBytes); err != nil {
		return domain.IndexResult{}, fmt.Errorf("save audio file: %w", err)
	}

	record, err := s.repo.InsertAudioRecord(
		ctx,
		originalFilename,
		storedFilename,
		mimeType,
		int64(len(audioBytes)),
		s.embedding.ModelName(),
		vector,
	)
	if err != nil {
		return domain.IndexResult{}, fmt.Errorf("store audio record: %w", err)
	}

	return domain.IndexResult{
		ID:                  record.ID,
		OriginalFilename:    record.OriginalFilename,
		StoredFilename:      record.StoredFilename,
		MIMEType:            record.MIMEType,
		FileSizeBytes:       record.FileSizeBytes,
		EmbeddingDimensions: record.EmbeddingDims,
	}, nil
}

// SearchByText embeds the query online and ranks stored audio by cosine similarity.
func (s *Service) SearchByText(ctx context.Context, text string, limit int) ([]domain.AudioRecord, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("text is required")
	}

	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	queryVector, err := s.embedding.EmbedText(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("gemini text embedding failed: %w", err)
	}

	stored, err := s.repo.ListAudioRecords(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]domain.AudioRecord, 0, len(stored))
	for _, item := range stored {
		record := item.Record
		record.SimilarityScore = cosineSimilarity(queryVector, item.Embedding)
		record.DownloadURL = fmt.Sprintf("/api/audio/%d", record.ID)
		results = append(results, record)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SimilarityScore > results[j].SimilarityScore
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetAudioRecord returns one stored audio record and fills its public download URL.
func (s *Service) GetAudioRecord(ctx context.Context, id int64) (domain.AudioRecord, error) {
	record, err := s.repo.GetAudioRecord(ctx, id)
	if err != nil {
		return domain.AudioRecord{}, err
	}
	record.DownloadURL = fmt.Sprintf("/api/audio/%d", record.ID)
	return record, nil
}

// AudioPath returns the absolute path of a stored audio file.
func (s *Service) AudioPath(storedFilename string) string {
	return s.storage.AudioPath(storedFilename)
}

// BuildStoredFilename generates a collision-resistant storage filename.
func BuildStoredFilename(original string) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(original))
	if ext == "" {
		ext = ".bin"
	}

	return fmt.Sprintf("%d-%s%s", time.Now().UTC().Unix(), hex.EncodeToString(random), ext), nil
}

// DetectMIMEType infers the MIME type from extension first, then content bytes.
func DetectMIMEType(filename string, body []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			return mimeType
		}
	}
	return http.DetectContentType(body)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var dot float64
	var normA float64
	var normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
