package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"kiria/backend/handler"
	"kiria/backend/infrastructure/config"
	"kiria/backend/infrastructure/gemini"
	"kiria/backend/infrastructure/sqlite"
	"kiria/backend/infrastructure/storage"
	"kiria/backend/usecase"
)

// main starts the online text-search API server.
func main() {
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

	httpHandler := handler.NewHTTPHandler(service)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpHandler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	log.Printf("backend listening on http://localhost%s", cfg.Addr)
	log.Printf("using Gemini embedding model %q", cfg.GeminiModel)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
