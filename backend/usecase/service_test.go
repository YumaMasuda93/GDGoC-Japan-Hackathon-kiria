package usecase

import (
	"context"
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

type stubMusicClient struct {
	output domain.MusicGenerationOutput
}

func (s stubMusicClient) GenerateMusic(_ context.Context, _ domain.MusicGenerationRequest) (domain.MusicGenerationOutput, error) {
	return s.output, nil
}

func (stubMusicClient) ModelName() string {
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
	service := NewServiceWithMusic(
		stubEmbeddingClient{},
		stubMusicClient{
			output: domain.MusicGenerationOutput{
				Model: "stub-lyria",
				Clips: []domain.GeneratedAudioSample{
					{
						MIMEType:  "audio/wav",
						AudioData: []byte("RIFF....WAVE"),
					},
				},
			},
		},
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
		stubMusicClient{},
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
