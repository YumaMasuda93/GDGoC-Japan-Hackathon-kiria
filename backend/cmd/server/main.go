package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"kiria/backend/handler"
	"kiria/backend/infrastructure/config"
	"kiria/backend/infrastructure/gemini"
	"kiria/backend/infrastructure/sqlite"
	"kiria/backend/infrastructure/storage"
	"kiria/backend/infrastructure/vertexai"
	"kiria/backend/usecase"
)

// main はオンラインのテキスト検索APIサーバーを起動します。
func main() {
	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}
	if cfg.GoogleCredentials == "" {
		log.Printf("GOOGLE_APPLICATION_CREDENTIALS is not set or could not be resolved")
	} else {
		log.Printf("using Google application credentials at %q", cfg.GoogleCredentials)
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
	outputStore, err := storage.NewFileStore(cfg.OutputDir)
	if err != nil {
		log.Fatal(err)
	}

	embeddingClient := gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModel)
	translator := gemini.NewTranslator(cfg.GeminiAPIKey, cfg.GeminiTextModel)
	service := usecase.NewService(embeddingClient, repo, fileStore)

	musicClient, err := vertexai.NewMusicClient(context.Background(), cfg.GoogleCloudProject, cfg.VertexLocation, cfg.LyriaModel)
	if err != nil {
		log.Printf("vertex ai music generation disabled: %v", err)
	} else {
		service = usecase.NewServiceWithMusicAndTranslator(embeddingClient, musicClient, translator, repo, fileStore)
		service.SetGeneratedOutputStorage(outputStore)
		log.Printf("using Vertex AI music model %q in %q", cfg.LyriaModel, cfg.VertexLocation)
	}

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
	log.Printf("using Gemini text model %q for prompt translation", cfg.GeminiTextModel)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
