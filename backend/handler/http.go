package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kiria/backend/domain"
	"kiria/backend/usecase"
)

type apiError struct {
	Error string `json:"error"`
}

// HTTPHandler exposes the online retrieval API over HTTP.
type HTTPHandler struct {
	service *usecase.Service
}

// NewHTTPHandler constructs the HTTP handler layer.
func NewHTTPHandler(service *usecase.Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// Routes builds the HTTP router for the online API.
func (h *HTTPHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.HealthHandler)
	mux.HandleFunc("/api/search/text", h.SearchTextHandler)
	mux.HandleFunc("/api/audio/", h.AudioFileHandler)
	return logRequest(mux)
}

// HealthHandler reports process health and the active embedding model.
func (h *HTTPHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, domain.HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Model:     h.service.ModelName(),
	})
}

// SearchTextHandler runs online text embedding plus similarity search.
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

	writeJSON(w, http.StatusOK, domain.SearchResponse{
		Query:   strings.TrimSpace(req.Text),
		Results: results,
	})
}

// AudioFileHandler streams a stored audio file back to the client.
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
	http.ServeFile(w, r, h.service.AudioPath(record.StoredFilename))
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
