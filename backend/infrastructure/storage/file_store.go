package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FileStore stores audio files on the local filesystem.
type FileStore struct {
	audioDir string
}

// NewFileStore creates the audio storage directory if it does not exist.
func NewFileStore(audioDir string) (*FileStore, error) {
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		return nil, fmt.Errorf("create audio dir: %w", err)
	}

	return &FileStore{audioDir: audioDir}, nil
}

// SaveAudio writes an audio file to the configured storage directory.
func (s *FileStore) SaveAudio(_ context.Context, storedFilename string, audioData []byte) error {
	return os.WriteFile(filepath.Join(s.audioDir, storedFilename), audioData, 0o644)
}

// AudioPath returns the absolute path to a stored audio file.
func (s *FileStore) AudioPath(storedFilename string) string {
	return filepath.Join(s.audioDir, storedFilename)
}
