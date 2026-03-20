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
	"strconv"
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
	output     domain.AudioStorage
}

// IndexAudioFileOptions は音声インデックス化の保存方式を制御します。
type IndexAudioFileOptions struct {
	ReferenceSourcePath bool
}

const uiGeneratedSourcePathPrefix = "ui-generated-"
const recitationSafeNegativePrompt = "lyrics, vocals, recognizable melody, direct song imitation, copyrighted motif, verbatim phrases"

// NewService はアプリケーションの中核ユースケースを構築します。
func NewService(embedding domain.EmbeddingClient, repo domain.AudioRepository, storage domain.AudioStorage) *Service {
	return &Service{
		embedding: embedding,
		repo:      repo,
		storage:   storage,
		output:    storage,
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

// SetGeneratedOutputStorage は最終生成音楽の保存先を差し替えます。
func (s *Service) SetGeneratedOutputStorage(storage domain.AudioStorage) {
	if storage != nil {
		s.output = storage
	}
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
	return s.IndexAudioFileWithOptions(ctx, sourcePath, IndexAudioFileOptions{})
}

// IndexAudioFileWithOptions はローカル音声を埋め込みし、保存方式を選んで登録します。
func (s *Service) IndexAudioFileWithOptions(ctx context.Context, sourcePath string, opts IndexAudioFileOptions) (domain.IndexResult, error) {
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

	storedSourcePath := NormalizeIndexedSourcePath(sourcePath)
	if !opts.ReferenceSourcePath {
		storedSourcePath, err = BuildStoredFilename(originalFilename)
		if err != nil {
			return domain.IndexResult{}, fmt.Errorf("build stored filename: %w", err)
		}

		if err := s.storage.SaveAudio(ctx, storedSourcePath, audioBytes); err != nil {
			return domain.IndexResult{}, fmt.Errorf("save indexed audio: %w", err)
		}
	}

	record, err := s.repo.InsertAudioRecord(
		ctx,
		originalFilename,
		storedSourcePath,
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

// NormalizeIndexedSourcePath はインデックス登録用のソースパスをなるべく相対化します。
func NormalizeIndexedSourcePath(sourcePath string) string {
	cleaned := filepath.Clean(sourcePath)
	if cleaned == "." {
		return sourcePath
	}
	if !filepath.IsAbs(cleaned) {
		return cleaned
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cleaned
	}

	resolvedSourcePath := cleaned
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil && resolved != "" {
		resolvedSourcePath = resolved
	}

	resolvedCWD := cwd
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil && resolved != "" {
		resolvedCWD = resolved
	}

	relative, err := filepath.Rel(resolvedCWD, resolvedSourcePath)
	if err != nil {
		return cleaned
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return cleaned
	}

	return filepath.Clean(relative)
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

		if isExcludedFromSearch(record.SourcePath) {
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

// GeneratedAudioPath は最終生成済み音声ファイルの実パスを返します。
func (s *Service) GeneratedAudioPath(sourcePath string) string {
	return s.output.AudioPath(sourcePath)
}

// GenerateMusic はプロンプトから音楽を生成して保存し、返却用メタデータを返します。
func (s *Service) GenerateMusic(ctx context.Context, req domain.MusicGenerationRequest) (domain.MusicGenerationResponse, error) {
	if s.music == nil {
		return domain.MusicGenerationResponse{}, errors.New("music generation is not configured")
	}

	selectedEmbeddings, err := s.selectedAudioEmbeddings(ctx, req.SelectedAudioIDs)
	if err != nil {
		return domain.MusicGenerationResponse{}, err
	}

	originalPrompt := strings.TrimSpace(req.Prompt)
	req.Prompt = originalPrompt

	translatedPrompt := originalPrompt
	if s.translator != nil && originalPrompt != "" {
		translatedPrompt, err = s.translator.TranslateToEnglish(ctx, originalPrompt)
		if err != nil {
			return domain.MusicGenerationResponse{}, fmt.Errorf("translate generation prompt failed: %w", err)
		}
		req.Prompt = translatedPrompt
	}

	req.Prompt = buildOriginalMusicPrompt(req.Prompt)
	if strings.TrimSpace(req.NegativePrompt) == "" {
		req.NegativePrompt = recitationSafeNegativePrompt
	}

	output, err := s.music.GenerateMusic(ctx, req)
	if err != nil {
		if isRecitationBlocked(err) {
			retriedReq := req
			retriedReq.Prompt = buildRecitationRetryPrompt(req.Prompt)
			output, err = s.music.GenerateMusic(ctx, retriedReq)
			if err == nil {
				req = retriedReq
				translatedPrompt = retriedReq.Prompt
			}
		}
	}
	if err != nil {
		return domain.MusicGenerationResponse{}, fmt.Errorf("lyria music generation failed: %w", err)
	}

	translatedPrompt = req.Prompt

	bestClip, err := s.pickBestGeneratedClip(ctx, output.Clips, selectedEmbeddings)
	if err != nil {
		return domain.MusicGenerationResponse{}, err
	}

	response := domain.MusicGenerationResponse{
		Prompt:           originalPrompt,
		TranslatedPrompt: translatedPrompt,
		NegativePrompt:   strings.TrimSpace(req.NegativePrompt),
		Model:            output.Model,
		ModelDisplay:     output.ModelDisplay,
		Clips:            make([]domain.GeneratedMusicClip, 0, 1),
	}

	if len(bestClip.AudioData) > 0 {
		savedClip, err := s.storeOutputClip(ctx, "lyria-generated-best"+extensionForMIME(bestClip.MIMEType), bestClip)
		if err != nil {
			return domain.MusicGenerationResponse{}, err
		}
		response.Clips = append(response.Clips, savedClip)
	}

	return response, nil
}

// StoreGeneratedClip は生成済み音声を保存し、埋め込みとメタデータを登録します。
func (s *Service) StoreGeneratedClip(ctx context.Context, originalFilename string, clip domain.GeneratedAudioSample) (domain.GeneratedMusicClip, error) {
	return s.storeGeneratedClip(ctx, originalFilename, clip, false)
}

func (s *Service) storeOutputClip(ctx context.Context, originalFilename string, clip domain.GeneratedAudioSample) (domain.GeneratedMusicClip, error) {
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

	if err := s.output.SaveAudio(ctx, storedFilename, clip.AudioData); err != nil {
		return domain.GeneratedMusicClip{}, fmt.Errorf("save generated audio: %w", err)
	}

	return domain.GeneratedMusicClip{
		Filename:      storedFilename,
		MIMEType:      clip.MIMEType,
		FileSizeBytes: int64(len(clip.AudioData)),
		DownloadURL:   fmt.Sprintf("/api/generated/%s", storedFilename),
	}, nil
}

func (s *Service) storeGeneratedClip(ctx context.Context, originalFilename string, clip domain.GeneratedAudioSample, excludeFromSearch bool) (domain.GeneratedMusicClip, error) {
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
	if excludeFromSearch {
		storedFilename = uiGeneratedSourcePathPrefix + storedFilename
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

func isExcludedFromSearch(sourcePath string) bool {
	return strings.HasPrefix(filepath.Base(sourcePath), uiGeneratedSourcePathPrefix)
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

func (s *Service) selectedAudioEmbeddings(ctx context.Context, ids []int64) ([][]float64, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	stored, err := s.repo.ListAudioRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("list selected audio records: %w", err)
	}

	byID := make(map[int64][]float64, len(stored))
	for _, item := range stored {
		byID[item.Record.ID] = item.Embedding
	}

	vectors := make([][]float64, 0, len(ids))
	missingIDs := make([]string, 0)
	for _, id := range uniqueInt64s(ids) {
		vector, ok := byID[id]
		if !ok {
			missingIDs = append(missingIDs, strconv.FormatInt(id, 10))
			continue
		}
		vectors = append(vectors, vector)
	}

	if len(missingIDs) > 0 {
		return nil, fmt.Errorf("selected audio ids not found: %s", strings.Join(missingIDs, ", "))
	}

	return vectors, nil
}

func (s *Service) pickBestGeneratedClip(ctx context.Context, clips []domain.GeneratedAudioSample, selectedEmbeddings [][]float64) (domain.GeneratedAudioSample, error) {
	if len(clips) == 0 {
		return domain.GeneratedAudioSample{}, nil
	}
	if len(selectedEmbeddings) == 0 {
		return clips[0], nil
	}

	bestIndex := 0
	bestScore := math.Inf(-1)

	for i, clip := range clips {
		score, err := s.generatedClipScore(ctx, clip, i, selectedEmbeddings)
		if err != nil {
			return domain.GeneratedAudioSample{}, err
		}
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	return clips[bestIndex], nil
}

func (s *Service) generatedClipScore(ctx context.Context, clip domain.GeneratedAudioSample, index int, selectedEmbeddings [][]float64) (float64, error) {
	vector, err := s.embedding.EmbedAudio(
		ctx,
		clip.MIMEType,
		clip.AudioData,
		fmt.Sprintf("lyria-generated-candidate-%d%s", index+1, extensionForMIME(clip.MIMEType)),
	)
	if err != nil {
		return 0, fmt.Errorf("embed generated audio candidate %d: %w", index+1, err)
	}

	var total float64
	for _, selected := range selectedEmbeddings {
		total += cosineSimilarity(vector, selected)
	}
	return total / float64(len(selectedEmbeddings)), nil
}

func uniqueInt64s(values []int64) []int64 {
	if len(values) <= 1 {
		return values
	}

	seen := make(map[int64]struct{}, len(values))
	unique := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}

	return unique
}

func buildOriginalMusicPrompt(prompt string) string {
	prompt = normalizePromptText(prompt)
	if prompt == "" {
		return prompt
	}

	return strings.Join([]string{
		"Compose a fully original instrumental track.",
		"Avoid recognizable melodies, quoted lyrics, and close resemblance to any existing song.",
		prompt,
	}, "\n")
}

func buildRecitationRetryPrompt(prompt string) string {
	prompt = normalizePromptText(prompt)
	if prompt == "" {
		return prompt
	}

	return strings.Join([]string{
		"Create a new, original instrumental piece using only high-level musical attributes.",
		"Do not imitate any existing song, artist, melody, hook, lyric, or arrangement.",
		"Describe mood, tempo, texture, rhythm, instrumentation, and scene in abstract terms only.",
		prompt,
	}, "\n")
}

func normalizePromptText(prompt string) string {
	replacer := strings.NewReplacer(
		"->", "\n",
		"→", "\n",
		"\"", "",
		"'",
		"",
		"“", "",
		"”", "",
		"‘", "",
		"’", "",
	)
	parts := strings.FieldsFunc(replacer.Replace(prompt), func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

func isRecitationBlocked(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "recitation") || strings.Contains(message, "blocked by recitation")
}
