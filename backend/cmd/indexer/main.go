package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"kiria/backend/infrastructure/config"
	"kiria/backend/infrastructure/gemini"
	"kiria/backend/infrastructure/sqlite"
	"kiria/backend/infrastructure/storage"
	"kiria/backend/usecase"
)

// main はローカル音声を事前埋め込みして SQLite に登録します。
func main() {
	var referenceSource bool
	var skipExisting bool
	var timeout time.Duration

	flag.BoolVar(&referenceSource, "reference", false, "store the original source path instead of copying into data/audio")
	flag.BoolVar(&skipExisting, "skip-existing", false, "skip files whose source path is already indexed (requires -reference)")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "per-file embedding timeout")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("usage: go run ./cmd/indexer [-reference] [-skip-existing] <audio-file-or-dir> [<audio-file-or-dir>...]")
	}
	if skipExisting && !referenceSource {
		log.Fatal("-skip-existing requires -reference")
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

	paths, err := expandInputPaths(flag.Args())
	if err != nil {
		log.Fatal(err)
	}
	if len(paths) == 0 {
		log.Fatal("no audio files found")
	}

	var failed bool
	for _, path := range paths {
		if skipExisting {
			indexedSourcePath := usecase.NormalizeIndexedSourcePath(path)
			exists, err := repo.HasSourcePath(indexedSourcePath)
			if err != nil {
				failed = true
				log.Printf("lookup failed for %s: %v", path, err)
				continue
			}
			if exists {
				log.Printf("skip indexed source=%s", indexedSourcePath)
				continue
			}
		}

		if err := indexFile(service, path, usecase.IndexAudioFileOptions{ReferenceSourcePath: referenceSource}, timeout); err != nil {
			failed = true
			log.Printf("index failed for %s: %v", path, err)
		}
	}

	if failed {
		os.Exit(1)
	}
}

// indexFile は音声1件を埋め込みし、登録結果をログ出力します。
func indexFile(service *usecase.Service, path string, opts usecase.IndexAudioFileOptions, timeout time.Duration) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return errors.New("directories are not supported")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := service.IndexAudioFileWithOptions(ctx, path, opts)
	if err != nil {
		return err
	}

	log.Printf(
		"indexed id=%d file=%s source=%s mime=%s bytes=%d dims=%d",
		result.ID,
		result.OriginalFilename,
		result.SourcePath,
		result.MIMEType,
		result.FileSizeBytes,
		result.EmbeddingDimensions,
	)
	return nil
}

func expandInputPaths(inputs []string) ([]string, error) {
	paths := make([]string, 0, len(inputs))
	seen := make(map[string]struct{})

	appendPath := func(path string) {
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		paths = append(paths, cleaned)
	}

	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			return nil, fmt.Errorf("stat input %s: %w", input, err)
		}

		if !info.IsDir() {
			if isAudioFile(input) {
				appendPath(input)
			}
			continue
		}

		err = filepath.WalkDir(input, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !isAudioFile(path) {
				return nil
			}
			appendPath(path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk input %s: %w", input, err)
		}
	}

	slices.Sort(paths)
	return paths, nil
}

func isAudioFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav", ".mp3", ".m4a", ".ogg", ".flac":
		return true
	default:
		return false
	}
}
