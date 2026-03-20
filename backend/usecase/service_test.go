package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiria/backend/domain"
	"kiria/backend/infrastructure/storage"
)

type stubEmbeddingClient struct{}

func (stubEmbeddingClient) EmbedText(_ context.Context, text string) ([]float64, error) {
	return []float64{float64(len(text))}, nil
}

func (stubEmbeddingClient) EmbedAudio(_ context.Context, _ string, _ []byte, _ string) ([]float64, error) {
	return []float64{1, 2, 3}, nil
}

func (stubEmbeddingClient) ModelName() string {
	return "stub-model"
}

type configurableEmbeddingClient struct {
	defaultAudioVector []float64
	audioVectors       map[string][]float64
}

func (c configurableEmbeddingClient) EmbedText(_ context.Context, text string) ([]float64, error) {
	return []float64{float64(len(text))}, nil
}

func (c configurableEmbeddingClient) EmbedAudio(_ context.Context, _ string, _ []byte, title string) ([]float64, error) {
	if vector, ok := c.audioVectors[title]; ok {
		return append([]float64(nil), vector...), nil
	}
	return append([]float64(nil), c.defaultAudioVector...), nil
}

func (configurableEmbeddingClient) ModelName() string {
	return "stub-model"
}

type stubTranslator struct {
	translated string
}

func (s stubTranslator) TranslateToEnglish(_ context.Context, _ string) (string, error) {
	return s.translated, nil
}

func (stubTranslator) ModelName() string {
	return "stub-translator"
}

type stubMusicClient struct {
	output   domain.MusicGenerationOutput
	outputs  []domain.MusicGenerationOutput
	errors   []error
	requests []domain.MusicGenerationRequest
}

func (s *stubMusicClient) GenerateMusic(_ context.Context, req domain.MusicGenerationRequest) (domain.MusicGenerationOutput, error) {
	s.requests = append(s.requests, req)
	callIndex := len(s.requests) - 1
	if callIndex < len(s.errors) && s.errors[callIndex] != nil {
		return domain.MusicGenerationOutput{}, s.errors[callIndex]
	}
	if callIndex < len(s.outputs) {
		return s.outputs[callIndex], nil
	}
	return s.output, nil
}

func (*stubMusicClient) ModelName() string {
	return "stub-music-model"
}

type recordingRepo struct {
	nextID      int64
	inserted    []domain.AudioRecord
	listed      []domain.StoredAudioEmbedding
	lastVector  []float64
	lastContext context.Context
}

func (r *recordingRepo) InsertAudioRecord(ctx context.Context, originalFilename, sourcePath, mimeType string, fileSizeBytes int64, embeddingModel string, embedding []float64) (domain.AudioRecord, error) {
	r.nextID++
	record := domain.AudioRecord{
		ID:               r.nextID,
		OriginalFilename: originalFilename,
		SourcePath:       sourcePath,
		MIMEType:         mimeType,
		FileSizeBytes:    fileSizeBytes,
		EmbeddingModel:   embeddingModel,
		EmbeddingDims:    len(embedding),
	}
	r.inserted = append(r.inserted, record)
	r.lastVector = append([]float64(nil), embedding...)
	r.lastContext = ctx
	return record, nil
}

func (*recordingRepo) GetAudioRecord(_ context.Context, _ int64) (domain.AudioRecord, error) {
	return domain.AudioRecord{}, nil
}

func (r *recordingRepo) ListAudioRecords(_ context.Context) ([]domain.StoredAudioEmbedding, error) {
	return append([]domain.StoredAudioEmbedding(nil), r.listed...), nil
}

func (*recordingRepo) Close() error {
	return nil
}

func TestIndexAudioFileStoresManagedRelativePath(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	sourcePath := filepath.Join(root, "input.wav")
	audioData := []byte("RIFF....WAVE")
	if err := os.WriteFile(sourcePath, audioData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	repo := &recordingRepo{}
	service := NewService(stubEmbeddingClient{}, repo, store)

	result, err := service.IndexAudioFile(context.Background(), sourcePath)
	if err != nil {
		t.Fatalf("IndexAudioFile() error = %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted count = %d, want 1", len(repo.inserted))
	}

	record := repo.inserted[0]
	if filepath.IsAbs(record.SourcePath) {
		t.Fatalf("SourcePath should be relative, got %q", record.SourcePath)
	}
	if filepath.Ext(record.SourcePath) != ".wav" {
		t.Fatalf("SourcePath ext = %q, want .wav", filepath.Ext(record.SourcePath))
	}

	storedPath := store.AudioPath(record.SourcePath)
	if !strings.HasPrefix(storedPath, audioDir) {
		t.Fatalf("stored path = %q, want prefix %q", storedPath, audioDir)
	}
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("stored file missing at %q: %v", storedPath, err)
	}
	if result.SourcePath != record.SourcePath {
		t.Fatalf("result.SourcePath = %q, want %q", result.SourcePath, record.SourcePath)
	}
}

func TestIndexAudioFileKeepsReferencedSourcePath(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	sourcePath := filepath.Join(root, "data", "all_datas_shuffle", "track_0001.wav")
	audioData := []byte("RIFF....WAVE")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, audioData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	repo := &recordingRepo{}
	service := NewService(stubEmbeddingClient{}, repo, store)

	result, err := service.IndexAudioFileWithOptions(context.Background(), sourcePath, IndexAudioFileOptions{
		ReferenceSourcePath: true,
	})
	if err != nil {
		t.Fatalf("IndexAudioFileWithOptions() error = %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted count = %d, want 1", len(repo.inserted))
	}

	record := repo.inserted[0]
	wantSourcePath := filepath.Join("data", "all_datas_shuffle", "track_0001.wav")
	if record.SourcePath != wantSourcePath {
		t.Fatalf("SourcePath = %q, want %q", record.SourcePath, wantSourcePath)
	}

	storedPath := store.AudioPath(record.SourcePath)
	if storedPath != wantSourcePath {
		t.Fatalf("stored path = %q, want %q", storedPath, wantSourcePath)
	}
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("stored file missing at %q: %v", storedPath, err)
	}

	entries, err := os.ReadDir(audioDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("audio dir entries = %d, want 0", len(entries))
	}
	if result.SourcePath != wantSourcePath {
		t.Fatalf("result.SourcePath = %q, want %q", result.SourcePath, wantSourcePath)
	}
}

func TestGenerateMusicStoresRelativeIndexedPath(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	outputDir := filepath.Join(root, "output")
	outputStore, err := storage.NewFileStore(outputDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	repo := &recordingRepo{}
	musicClient := &stubMusicClient{
		output: domain.MusicGenerationOutput{
			Model: "stub-lyria",
			Clips: []domain.GeneratedAudioSample{
				{
					MIMEType:  "audio/wav",
					AudioData: []byte("RIFF....WAVE"),
				},
			},
		},
	}
	service := NewServiceWithMusic(
		stubEmbeddingClient{},
		musicClient,
		repo,
		store,
	)
	service.SetGeneratedOutputStorage(outputStore)

	resp, err := service.GenerateMusic(context.Background(), domain.MusicGenerationRequest{
		Prompt:      "test",
		SampleCount: 1,
	})
	if err != nil {
		t.Fatalf("GenerateMusic() error = %v", err)
	}
	if len(repo.inserted) != 0 {
		t.Fatalf("inserted count = %d, want 0", len(repo.inserted))
	}

	if filepath.IsAbs(resp.Clips[0].Filename) {
		t.Fatalf("Filename should be relative, got %q", resp.Clips[0].Filename)
	}
	if len(resp.Clips) != 1 {
		t.Fatalf("clips count = %d, want 1", len(resp.Clips))
	}
	if resp.Clips[0].IndexedAudioID != nil {
		t.Fatalf("IndexedAudioID = %v, want nil", *resp.Clips[0].IndexedAudioID)
	}
	if resp.Clips[0].IndexedAudioURL != "" {
		t.Fatalf("IndexedAudioURL = %q, want empty", resp.Clips[0].IndexedAudioURL)
	}

	storedPath := outputStore.AudioPath(resp.Clips[0].Filename)
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("stored file missing at %q: %v", storedPath, err)
	}
	if strings.HasPrefix(storedPath, audioDir) {
		t.Fatalf("stored path = %q, should not be under %q", storedPath, audioDir)
	}
	if len(musicClient.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(musicClient.requests))
	}
	if musicClient.requests[0].SampleCount != 1 {
		t.Fatalf("SampleCount = %d, want 1", musicClient.requests[0].SampleCount)
	}
}

func TestStoreGeneratedClipKeepsSearchablePath(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	repo := &recordingRepo{}
	service := NewServiceWithMusic(
		stubEmbeddingClient{},
		&stubMusicClient{},
		repo,
		store,
	)

	clip, err := service.StoreGeneratedClip(context.Background(), "lyria-batch-001.wav", domain.GeneratedAudioSample{
		MIMEType:  "audio/wav",
		AudioData: []byte("RIFF....WAVE"),
	})
	if err != nil {
		t.Fatalf("StoreGeneratedClip() error = %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted count = %d, want 1", len(repo.inserted))
	}

	record := repo.inserted[0]
	if strings.HasPrefix(record.SourcePath, uiGeneratedSourcePathPrefix) {
		t.Fatalf("SourcePath = %q, should not have UI prefix", record.SourcePath)
	}
	if clip.Filename != record.SourcePath {
		t.Fatalf("clip.Filename = %q, want %q", clip.Filename, record.SourcePath)
	}
}

func TestGenerateMusicRanksCandidatesBySelectedAudio(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	outputDir := filepath.Join(root, "output")
	outputStore, err := storage.NewFileStore(outputDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	repo := &recordingRepo{
		listed: []domain.StoredAudioEmbedding{
			{
				Record:    domain.AudioRecord{ID: 11, SourcePath: "selected-a.wav"},
				Embedding: []float64{1, 0},
			},
			{
				Record:    domain.AudioRecord{ID: 22, SourcePath: "selected-b.wav"},
				Embedding: []float64{0, 1},
			},
		},
	}
	musicClient := &stubMusicClient{
		output: domain.MusicGenerationOutput{
			Model: "stub-lyria",
			Clips: []domain.GeneratedAudioSample{
				{MIMEType: "audio/wav", AudioData: []byte("candidate-1")},
				{MIMEType: "audio/wav", AudioData: []byte("candidate-2")},
				{MIMEType: "audio/wav", AudioData: []byte("candidate-3")},
				{MIMEType: "audio/wav", AudioData: []byte("candidate-4")},
			},
		},
	}
	service := NewServiceWithMusic(
		configurableEmbeddingClient{
			defaultAudioVector: []float64{1, 0},
			audioVectors: map[string][]float64{
				"lyria-generated-candidate-1.wav": {1, 0},
				"lyria-generated-candidate-2.wav": {0, 1},
				"lyria-generated-candidate-3.wav": {0.8, 0.8},
				"lyria-generated-candidate-4.wav": {0.2, 0},
			},
		},
		musicClient,
		repo,
		store,
	)
	service.SetGeneratedOutputStorage(outputStore)

	resp, err := service.GenerateMusic(context.Background(), domain.MusicGenerationRequest{
		Prompt:           "test",
		SampleCount:      4,
		SelectedAudioIDs: []int64{11, 22},
	})
	if err != nil {
		t.Fatalf("GenerateMusic() error = %v", err)
	}
	if len(resp.Clips) != 1 {
		t.Fatalf("clips count = %d, want 1", len(resp.Clips))
	}
	if len(musicClient.requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(musicClient.requests))
	}
	if musicClient.requests[0].SampleCount != 4 {
		t.Fatalf("SampleCount = %d, want 4", musicClient.requests[0].SampleCount)
	}
	if len(musicClient.requests[0].SelectedAudioIDs) != 2 {
		t.Fatalf("SelectedAudioIDs count = %d, want 2", len(musicClient.requests[0].SelectedAudioIDs))
	}

	storedPath := outputStore.AudioPath(resp.Clips[0].Filename)
	audioData, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", storedPath, err)
	}
	if string(audioData) != "candidate-3" {
		t.Fatalf("stored audio = %q, want candidate-3", string(audioData))
	}
}

func TestGenerateMusicRetriesRecitationBlockedPrompt(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	outputDir := filepath.Join(root, "output")
	outputStore, err := storage.NewFileStore(outputDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	repo := &recordingRepo{}
	musicClient := &stubMusicClient{
		errors: []error{
			errors.New("status 400: Audio generation failed with the following error: All responses were blocked by recitation checks."),
			nil,
		},
		outputs: []domain.MusicGenerationOutput{
			{},
			{
				Model: "stub-lyria",
				Clips: []domain.GeneratedAudioSample{
					{MIMEType: "audio/wav", AudioData: []byte("retry-success")},
				},
			},
		},
	}
	service := NewServiceWithMusicAndTranslator(
		stubEmbeddingClient{},
		musicClient,
		stubTranslator{translated: "midnight express with bright synth pulses"},
		repo,
		store,
	)
	service.SetGeneratedOutputStorage(outputStore)

	resp, err := service.GenerateMusic(context.Background(), domain.MusicGenerationRequest{
		Prompt:      "「真夜中の高速道路」みたいな感じ",
		SampleCount: 4,
	})
	if err != nil {
		t.Fatalf("GenerateMusic() error = %v", err)
	}
	if len(resp.Clips) != 1 {
		t.Fatalf("clips count = %d, want 1", len(resp.Clips))
	}
	if len(musicClient.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(musicClient.requests))
	}
	if !strings.Contains(musicClient.requests[0].Prompt, "Compose a fully original instrumental track.") {
		t.Fatalf("first prompt = %q, want originality guard", musicClient.requests[0].Prompt)
	}
	if !strings.Contains(musicClient.requests[1].Prompt, "Create a new, original instrumental piece using only high-level musical attributes.") {
		t.Fatalf("second prompt = %q, want retry guard", musicClient.requests[1].Prompt)
	}
	if musicClient.requests[1].NegativePrompt != recitationSafeNegativePrompt {
		t.Fatalf("NegativePrompt = %q, want %q", musicClient.requests[1].NegativePrompt, recitationSafeNegativePrompt)
	}
	if resp.TranslatedPrompt != musicClient.requests[1].Prompt {
		t.Fatalf("TranslatedPrompt = %q, want %q", resp.TranslatedPrompt, musicClient.requests[1].Prompt)
	}
}

func TestSearchByTextExcludesUIGeneratedAudio(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")
	store, err := storage.NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	repo := &recordingRepo{
		listed: []domain.StoredAudioEmbedding{
			{
				Record: domain.AudioRecord{
					ID:               1,
					OriginalFilename: "lyria-batch-001.wav",
					SourcePath:       "1773919036-d4279734eaa3315e.wav",
					MIMEType:         "audio/wav",
				},
				Embedding: []float64{1},
			},
			{
				Record: domain.AudioRecord{
					ID:               2,
					OriginalFilename: "lyria-generated-1.wav",
					SourcePath:       uiGeneratedSourcePathPrefix + "1773919999-aabbccddeeff0011.wav",
					MIMEType:         "audio/wav",
				},
				Embedding: []float64{1},
			},
		},
	}
	service := NewService(stubEmbeddingClient{}, repo, store)

	results, err := service.SearchByText(context.Background(), "a", 5)
	if err != nil {
		t.Fatalf("SearchByText() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}
	if results[0].ID != 1 {
		t.Fatalf("result ID = %d, want 1", results[0].ID)
	}
}
