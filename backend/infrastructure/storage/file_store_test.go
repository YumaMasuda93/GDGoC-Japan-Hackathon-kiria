package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAudioPathResolvesManagedRelativeFile(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")

	store, err := NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	filename := "managed.wav"
	expected := filepath.Join(audioDir, filename)
	if err := os.WriteFile(expected, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := store.AudioPath(filename); got != expected {
		t.Fatalf("AudioPath() = %q, want %q", got, expected)
	}
}

func TestAudioPathKeepsExistingRelativePath(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")

	store, err := NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	sourcePath := filepath.Join("data", "audio", "generated.wav")
	expected := filepath.Join(root, sourcePath)
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(expected, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	if got := store.AudioPath(sourcePath); got != sourcePath {
		t.Fatalf("AudioPath() = %q, want %q", got, sourcePath)
	}
}

func TestAudioPathFallsBackToSeedBasename(t *testing.T) {
	root := t.TempDir()
	audioDir := filepath.Join(root, "data", "audio")

	store, err := NewFileStore(audioDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	filename := "sample.wav"
	expected := filepath.Join(root, "seed", filename)
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(expected, []byte("audio"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	missingLegacyPath := filepath.Join(root, "other-machine", filename)
	if got := store.AudioPath(missingLegacyPath); got != expected {
		t.Fatalf("AudioPath() = %q, want %q", got, expected)
	}
}
