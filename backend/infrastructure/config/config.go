package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultAddr           = ":8080"
	defaultDataDir        = "data"
	defaultAudioDir       = "audio"
	defaultDBPath         = "kiria.db"
	defaultEmbedModel     = "gemini-embedding-2-preview"
	defaultMaxUploadBytes = 25 << 20
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Addr           string
	DataDir        string
	AudioDir       string
	DBPath         string
	GeminiAPIKey   string
	GeminiModel    string
	MaxUploadBytes int64
}

// Load builds Config from environment variables and defaults.
func Load() Config {
	dataDir := getenv("DATA_DIR", defaultDataDir)
	addr := getenv("PORT", defaultAddr)
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	return Config{
		Addr:           addr,
		DataDir:        dataDir,
		AudioDir:       filepath.Join(dataDir, defaultAudioDir),
		DBPath:         filepath.Join(dataDir, defaultDBPath),
		GeminiAPIKey:   os.Getenv("GEMINI_API_KEY"),
		GeminiModel:    getenv("GEMINI_EMBED_MODEL", defaultEmbedModel),
		MaxUploadBytes: getInt64Env("MAX_UPLOAD_BYTES", defaultMaxUploadBytes),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt64Env(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
