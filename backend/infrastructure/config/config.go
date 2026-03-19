package config

import (
	"bufio"
	"errors"
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
	defaultVertexLocation = "us-central1"
	defaultLyriaModel     = "lyria-002"
	defaultMaxUploadBytes = 25 << 20
)

// Config は環境変数と `.env` から読み込んだ実行設定を表します。
type Config struct {
	Addr               string
	DataDir            string
	AudioDir           string
	DBPath             string
	GeminiAPIKey       string
	GeminiModel        string
	GoogleCloudProject string
	VertexLocation     string
	LyriaModel         string
	MaxUploadBytes     int64
}

// Load は `.env` と環境変数、既定値から Config を組み立てます。
func Load() Config {
	loadRuntimeEnv()

	dataDir := getenv("DATA_DIR", defaultDataDir)
	addr := getenv("PORT", defaultAddr)
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	return Config{
		Addr:               addr,
		DataDir:            dataDir,
		AudioDir:           filepath.Join(dataDir, defaultAudioDir),
		DBPath:             filepath.Join(dataDir, defaultDBPath),
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		GeminiModel:        getenv("GEMINI_EMBED_MODEL", defaultEmbedModel),
		GoogleCloudProject: getenv("GOOGLE_CLOUD_PROJECT", ""),
		VertexLocation:     getenv("VERTEX_AI_LOCATION", defaultVertexLocation),
		LyriaModel:         getenv("VERTEX_LYRIA_MODEL", defaultLyriaModel),
		MaxUploadBytes:     getInt64Env("MAX_UPLOAD_BYTES", defaultMaxUploadBytes),
	}
}

func loadRuntimeEnv() {
	candidates := []string{
		".env",
		filepath.Join(executableDir(), ".env"),
	}

	loaded := make(map[string]struct{})
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if _, seen := loaded[cleaned]; seen {
			continue
		}
		loaded[cleaned] = struct{}{}
		loadDotEnv(cleaned)
	}
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func executableDir() string {
	executablePath, err := os.Executable()
	if err != nil {
		return "."
	}

	dir := filepath.Dir(executablePath)
	if dir == "" {
		return "."
	}

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return dir
	}
	if resolved == "" {
		return dir
	}
	return resolved
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
