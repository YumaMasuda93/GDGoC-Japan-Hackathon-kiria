package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"kiria/backend/infrastructure/config"
	"kiria/backend/infrastructure/gemini"
	"kiria/backend/infrastructure/sqlite"
	"kiria/backend/infrastructure/storage"
	"kiria/backend/usecase"
)

// main はローカル音声を事前埋め込みして SQLite に登録します。
func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run ./cmd/indexer <audio-file> [<audio-file>...]")
	}

	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}

	repo, err := sqlite.NewAudioRepository(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	fileStore, err := storage.NewFileStore(cfg.AudioDir)
	if err != nil {
		log.Fatal(err)
	}

	service := usecase.NewService(
		gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModel),
		repo,
		fileStore,
	)

	var failed bool
	for _, path := range os.Args[1:] {
		if err := indexFile(service, path); err != nil {
			failed = true
			log.Printf("index failed for %s: %v", path, err)
		}
	}

	if failed {
		os.Exit(1)
	}
}

// indexFile は音声1件を埋め込みし、登録結果をログ出力します。
func indexFile(service *usecase.Service, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return errors.New("directories are not supported")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := service.IndexAudioFile(ctx, path)
	if err != nil {
		return err
	}

	log.Printf(
		"indexed id=%d file=%s stored=%s mime=%s bytes=%d dims=%d",
		result.ID,
		result.OriginalFilename,
		result.StoredFilename,
		result.MIMEType,
		result.FileSizeBytes,
		result.EmbeddingDimensions,
	)
	return nil
}
