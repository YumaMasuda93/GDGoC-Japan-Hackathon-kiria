package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	for _, candidate := range s.audioPathCandidates(sourcePath) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	candidates := s.audioPathCandidates(sourcePath)
	if len(candidates) == 0 {
		return filepath.Join(s.audioDir, sourcePath)
	}
	return candidates[len(candidates)-1]
}

// IsSeedFile は指定された元のファイル名が seed ディレクトリに存在し、かつ .wav であるかを判定します。
func (s *FileStore) IsSeedFile(originalFilename string) bool {
	if !strings.HasSuffix(strings.ToLower(originalFilename), ".wav") {
		return false
	}
	seedPath := filepath.Join(filepath.Dir(s.audioDir), "..", "seed", originalFilename)
	info, err := os.Stat(seedPath)
	return err == nil && !info.IsDir()
}

func (s *FileStore) audioPathCandidates(sourcePath string) []string {
	if sourcePath == "" {
		return []string{filepath.Join(s.audioDir, sourcePath)}
	}

	candidates := make([]string, 0, 5)
	seen := make(map[string]struct{})
	appendCandidate := func(path string) {
		if path == "" {
			return
		}

		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	if filepath.IsAbs(sourcePath) {
		appendCandidate(sourcePath)
	} else {
		appendCandidate(sourcePath)
		appendCandidate(filepath.Join(s.audioDir, sourcePath))
	}

	base := filepath.Base(sourcePath)
	if base != "." && base != string(filepath.Separator) {
		appendCandidate(filepath.Join(s.audioDir, base))
		appendCandidate(filepath.Join(filepath.Dir(s.audioDir), "..", "seed", base))
	}

	return candidates
}
