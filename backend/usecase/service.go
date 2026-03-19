package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
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
	embedding  domain.EmbeddingClient
	music      domain.MusicGenerationClient
	translator domain.PromptTranslator
	repo       domain.AudioRepository
	storage    domain.AudioStorage
}

// NewService はアプリケーションの中核ユースケースを構築します。
func NewService(embedding domain.EmbeddingClient, repo domain.AudioRepository, storage domain.AudioStorage) *Service {
	return &Service{
		embedding: embedding,
		repo:      repo,
		storage:   storage,
	}
}

// NewServiceWithMusic は音楽生成クライアント付きでユースケースを構築します。
func NewServiceWithMusic(embedding domain.EmbeddingClient, music domain.MusicGenerationClient, repo domain.AudioRepository, storage domain.AudioStorage) *Service {
	service := NewService(embedding, repo, storage)
	service.music = music
	return service
}

// NewServiceWithMusicAndTranslator は音楽生成と翻訳クライアント付きでユースケースを構築します。
func NewServiceWithMusicAndTranslator(embedding domain.EmbeddingClient, music domain.MusicGenerationClient, translator domain.PromptTranslator, repo domain.AudioRepository, storage domain.AudioStorage) *Service {
	service := NewServiceWithMusic(embedding, music, repo, storage)
	service.translator = translator
	return service
}

// ModelName は現在利用中の埋め込みモデル名を返します。
func (s *Service) ModelName() string {
	return s.embedding.ModelName()
}

// MusicModelName は現在利用中の音楽生成モデル名を返します。
func (s *Service) MusicModelName() string {
	if s.music == nil {
		return ""
	}
	return s.music.ModelName()
}

// Close はユースケースが利用する永続化資源を解放します。
func (s *Service) Close() error {
	return s.repo.Close()
}

// IndexAudioFile はローカル音声を埋め込みし、元ファイル参照とベクトルを保存します。
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
		return domain.IndexResult{}, fmt.Errorf("save indexed audio: %w", err)
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
		SourcePath:          record.SourcePath,
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

	results := make([]domain.AudioRecord, 0)
	for _, item := range stored {
		record := item.Record
		
		if !s.storage.IsSeedFile(record.OriginalFilename) {
			continue
		}

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
func (s *Service) AudioPath(sourcePath string) string {
	return s.storage.AudioPath(sourcePath)
}

// GenerateMusic はプロンプトから音楽を生成して保存し、返却用メタデータを返します。
func (s *Service) GenerateMusic(ctx context.Context, req domain.MusicGenerationRequest) (domain.MusicGenerationResponse, error) {
	if s.music == nil {
		return domain.MusicGenerationResponse{}, errors.New("music generation is not configured")
	}

	originalPrompt := strings.TrimSpace(req.Prompt)
	req.Prompt = originalPrompt

	translatedPrompt := originalPrompt
	var err error
	if s.translator != nil && originalPrompt != "" {
		translatedPrompt, err = s.translator.TranslateToEnglish(ctx, originalPrompt)
		if err != nil {
			return domain.MusicGenerationResponse{}, fmt.Errorf("translate generation prompt failed: %w", err)
		}
		req.Prompt = translatedPrompt
	}

	output, err := s.music.GenerateMusic(ctx, req)
	if err != nil {
		return domain.MusicGenerationResponse{}, fmt.Errorf("lyria music generation failed: %w", err)
	}

	response := domain.MusicGenerationResponse{
		Prompt:           originalPrompt,
		TranslatedPrompt: translatedPrompt,
		NegativePrompt:   strings.TrimSpace(req.NegativePrompt),
		Model:            output.Model,
		ModelDisplay:     output.ModelDisplay,
		Clips:            make([]domain.GeneratedMusicClip, 0, len(output.Clips)),
	}

	for i, clip := range output.Clips {
		originalFilename := fmt.Sprintf("lyria-generated-%d%s", i+1, extensionForMIME(clip.MIMEType))
		savedClip, err := s.StoreGeneratedClip(ctx, originalFilename, clip)
		if err != nil {
			if savedClip.Filename == "" {
				return domain.MusicGenerationResponse{}, err
			}
			log.Printf("skip indexing generated audio %q: %v", savedClip.Filename, err)
		}
		response.Clips = append(response.Clips, savedClip)
	}

	return response, nil
}

// StoreGeneratedClip は生成済み音声を保存し、埋め込みとメタデータを登録します。
func (s *Service) StoreGeneratedClip(ctx context.Context, originalFilename string, clip domain.GeneratedAudioSample) (domain.GeneratedMusicClip, error) {
	originalFilename = strings.TrimSpace(originalFilename)
	if originalFilename == "" {
		originalFilename = "lyria-generated" + extensionForMIME(clip.MIMEType)
	} else if filepath.Ext(originalFilename) == "" {
		originalFilename += extensionForMIME(clip.MIMEType)
	}

	storedFilename, err := BuildStoredFilename(originalFilename)
	if err != nil {
		return domain.GeneratedMusicClip{}, fmt.Errorf("build stored filename: %w", err)
	}

	if err := s.storage.SaveAudio(ctx, storedFilename, clip.AudioData); err != nil {
		return domain.GeneratedMusicClip{}, fmt.Errorf("save generated audio: %w", err)
	}

	savedClip := domain.GeneratedMusicClip{
		Filename:      storedFilename,
		MIMEType:      clip.MIMEType,
		FileSizeBytes: int64(len(clip.AudioData)),
		DownloadURL:   fmt.Sprintf("/api/generated/%s", storedFilename),
	}

	vector, err := s.embedding.EmbedAudio(ctx, clip.MIMEType, clip.AudioData, originalFilename)
	if err != nil {
		return savedClip, fmt.Errorf("embed generated audio: %w", err)
	}

	record, err := s.repo.InsertAudioRecord(
		ctx,
		originalFilename,
		storedFilename,
		clip.MIMEType,
		int64(len(clip.AudioData)),
		s.embedding.ModelName(),
		vector,
	)
	if err != nil {
		return savedClip, fmt.Errorf("store generated audio record: %w", err)
	}

	savedClip.IndexedAudioID = &record.ID
	savedClip.IndexedAudioURL = fmt.Sprintf("/api/audio/%d", record.ID)
	return savedClip, nil
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

func extensionForMIME(mimeType string) string {
	if mimeType != "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			return exts[0]
		}
	}

	switch mimeType {
	case "audio/wav", "audio/x-wav":
		return ".wav"
	default:
		return ".bin"
	}
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
