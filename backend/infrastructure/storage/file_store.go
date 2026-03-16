package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FileStore は音声ファイル本体をローカルファイルシステムに保存します。
type FileStore struct {
	audioDir string
}

// NewFileStore は音声保存ディレクトリを準備します。
func NewFileStore(audioDir string) (*FileStore, error) {
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		return nil, fmt.Errorf("create audio dir: %w", err)
	}

	return &FileStore{audioDir: audioDir}, nil
}

// SaveAudio は音声ファイルを保存先ディレクトリへ書き込みます。
func (s *FileStore) SaveAudio(_ context.Context, storedFilename string, audioData []byte) error {
	return os.WriteFile(filepath.Join(s.audioDir, storedFilename), audioData, 0o644)
}

// AudioPath は保存済み音声ファイルのパスを返します。
func (s *FileStore) AudioPath(storedFilename string) string {
	return filepath.Join(s.audioDir, storedFilename)
}
