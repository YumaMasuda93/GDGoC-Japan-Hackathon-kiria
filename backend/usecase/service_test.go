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

func (*recordingRepo) ListAudioRecords(_ context.Context) ([]domain.StoredAudioEmbedding, error) {
	return nil, nil
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

	resp, err := service.GenerateMusic(context.Background(), domain.MusicGenerationRequest{
		Prompt:      "test",
		SampleCount: 1,
	})
	if err != nil {
		t.Fatalf("GenerateMusic() error = %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted count = %d, want 1", len(repo.inserted))
	}

	record := repo.inserted[0]
	if filepath.IsAbs(record.SourcePath) {
		t.Fatalf("SourcePath should be relative, got %q", record.SourcePath)
	}
	if len(resp.Clips) != 1 {
		t.Fatalf("clips count = %d, want 1", len(resp.Clips))
	}
	if got := resp.Clips[0].Filename; got != record.SourcePath {
		t.Fatalf("resp.Clips[0].Filename = %q, want %q", got, record.SourcePath)
	}

	storedPath := store.AudioPath(record.SourcePath)
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("stored file missing at %q: %v", storedPath, err)
	}
}
