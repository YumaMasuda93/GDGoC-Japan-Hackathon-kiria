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

// Service は事前埋め込みとオンライン検索のユースケースをまとめます。
type Service struct {
	embedding domain.EmbeddingClient
	repo      domain.AudioRepository
	storage   domain.AudioStorage
}

// NewService はアプリケーションの中核ユースケースを構築します。
func NewService(embedding domain.EmbeddingClient, repo domain.AudioRepository, storage domain.AudioStorage) *Service {
	return &Service{
		embedding: embedding,
		repo:      repo,
		storage:   storage,
	}
}

// ModelName は現在利用中の埋め込みモデル名を返します。
func (s *Service) ModelName() string {
	return s.embedding.ModelName()
}

// Close はユースケースが利用する永続化資源を解放します。
func (s *Service) Close() error {
	return s.repo.Close()
}

// IndexAudioFile はローカル音声を埋め込みしてファイルとベクトルを保存します。
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

// SearchByText はクエリをオンライン埋め込みし、近い音声を類似度順で返します。
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

// GetAudioRecord は保存済み音声1件を取得し、公開用URLを補完します。
func (s *Service) GetAudioRecord(ctx context.Context, id int64) (domain.AudioRecord, error) {
	record, err := s.repo.GetAudioRecord(ctx, id)
	if err != nil {
		return domain.AudioRecord{}, err
	}
	record.DownloadURL = fmt.Sprintf("/api/audio/%d", record.ID)
	return record, nil
}

// AudioPath は保存済み音声ファイルの実パスを返します。
func (s *Service) AudioPath(storedFilename string) string {
	return s.storage.AudioPath(storedFilename)
}

// BuildStoredFilename は衝突しにくい保存用ファイル名を生成します。
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

// DetectMIMEType は拡張子を優先し、必要なら内容から MIME type を推定します。
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
