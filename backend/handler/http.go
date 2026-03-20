package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"kiria/backend/domain"
	"kiria/backend/usecase"
)

type apiError struct {
	Error string `json:"error"`
}

// HTTPHandler はオンライン検索APIを HTTP として公開します。
type HTTPHandler struct {
	service *usecase.Service
}

// NewHTTPHandler は HTTP ハンドラ層を生成します。
func NewHTTPHandler(service *usecase.Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// Routes はオンラインAPI用のルーティングを組み立てます。
func (h *HTTPHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.HealthHandler)
	mux.HandleFunc("/api/search/text", h.SearchTextHandler)
	mux.HandleFunc("/api/audio/", h.AudioFileHandler)
	mux.HandleFunc("/api/generated/", h.GeneratedAudioFileHandler)
	mux.HandleFunc("/api/music/generate", h.GenerateMusicHandler)
	return logRequest(mux)
}

// HealthHandler は稼働状態と利用中モデルを返します。
func (h *HTTPHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, domain.HealthResponse{
		Status:     "ok",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Model:      h.service.ModelName(),
		MusicModel: h.service.MusicModelName(),
	})
}

// SearchTextHandler はテキスト埋め込みと類似検索を実行します。
func (h *HTTPHandler) SearchTextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req domain.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	results, err := h.service.SearchByText(ctx, req.Text, req.Limit)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "text is required") {
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "gemini text embedding failed") {
			status = http.StatusBadGateway
		}
		writeError(w, status, err.Error())
		return
	}

	items := make([]domain.SearchResultItem, 0, len(results))
	for _, result := range results {
		items = append(items, domain.SearchResultItem{
			ID:               result.ID,
			OriginalFilename: result.OriginalFilename,
			MIMEType:         result.MIMEType,
			FileSizeBytes:    result.FileSizeBytes,
			EmbeddingModel:   result.EmbeddingModel,
			EmbeddingDims:    result.EmbeddingDims,
			SimilarityScore:  result.SimilarityScore,
			DownloadURL:      result.DownloadURL,
		})
	}

	writeJSON(w, http.StatusOK, domain.SearchResponse{
		Query:   strings.TrimSpace(req.Text),
		Results: items,
	})
}

// AudioFileHandler は保存済み音声ファイルを返します。
func (h *HTTPHandler) AudioFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/audio/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid audio id")
		return
	}

	record, err := h.service.GetAudioRecord(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "audio not found")
			return
		}

		writeError(w, http.StatusInternalServerError, fmt.Sprintf("load audio record: %v", err))
		return
	}

	w.Header().Set("Content-Type", record.MIMEType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", record.OriginalFilename))
	http.ServeFile(w, r, h.service.AudioPath(record.SourcePath))
}

// GeneratedAudioFileHandler は生成済み音声ファイルをファイル名で返します。
func (h *HTTPHandler) GeneratedAudioFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sourcePath := strings.TrimPrefix(r.URL.Path, "/api/generated/")
	if sourcePath == "" || sourcePath != path.Base(sourcePath) || sourcePath == "." {
		writeError(w, http.StatusBadRequest, "invalid generated audio filename")
		return
	}

	http.ServeFile(w, r, h.service.GeneratedAudioPath(sourcePath))
}

// GenerateMusicHandler は Lyria による音楽生成を実行します。
func (h *HTTPHandler) GenerateMusicHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req domain.MusicGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	resp, err := h.service.GenerateMusic(ctx, req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case strings.Contains(err.Error(), "prompt is required"),
			strings.Contains(err.Error(), "sampleCount"),
			strings.Contains(err.Error(), "seed cannot be used"),
			strings.Contains(err.Error(), "selected audio ids not found"):
			status = http.StatusBadRequest
		case strings.Contains(err.Error(), "music generation is not configured"):
			status = http.StatusServiceUnavailable
		case strings.Contains(err.Error(), "lyria music generation failed"):
			status = http.StatusBadGateway
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
