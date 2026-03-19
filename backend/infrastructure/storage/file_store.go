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
func (s *FileStore) SaveAudio(_ context.Context, sourcePath string, audioData []byte) error {
	targetPath := sourcePath
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(s.audioDir, targetPath)
	}
	return os.WriteFile(targetPath, audioData, 0o644)
}

// AudioPath は保存済み音声ファイルのパスを返します。
func (s *FileStore) AudioPath(sourcePath string) string {
	if filepath.IsAbs(sourcePath) {
		return sourcePath
	}
	return filepath.Join(s.audioDir, sourcePath)
}
