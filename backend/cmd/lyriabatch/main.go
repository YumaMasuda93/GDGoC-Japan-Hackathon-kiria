package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"kiria/backend/domain"
	"kiria/backend/infrastructure/config"
	"kiria/backend/infrastructure/gemini"
	"kiria/backend/infrastructure/sqlite"
	"kiria/backend/infrastructure/storage"
	"kiria/backend/infrastructure/vertexai"
	"kiria/backend/usecase"
)

const (
	defaultTrackCount      = 100
	defaultPromptBatchSize = 10
	defaultParallelism     = 10
	maxGenerateAttempts    = 220
)

var nonSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

type batchManifest struct {
	RunID       string               `json:"runId"`
	GeneratedAt string               `json:"generatedAt"`
	Requested   int                  `json:"requested"`
	Saved       int                  `json:"saved"`
	AudioDir    string               `json:"audioDir"`
	DBPath      string               `json:"dbPath"`
	Manifest    string               `json:"manifest"`
	PromptModel string               `json:"promptModel"`
	EmbedModel  string               `json:"embedModel"`
	MusicModel  string               `json:"musicModel"`
	Parallelism int                  `json:"parallelism"`
	Tracks      []batchManifestTrack `json:"tracks"`
	Errors      []string             `json:"errors,omitempty"`
}

type batchManifestTrack struct {
	Index           int      `json:"index"`
	Title           string   `json:"title"`
	Genre           string   `json:"genre"`
	Mood            string   `json:"mood"`
	Tags            []string `json:"tags"`
	Prompt          string   `json:"prompt"`
	NegativePrompt  string   `json:"negativePrompt"`
	Filename        string   `json:"filename"`
	RelativePath    string   `json:"relativePath"`
	AbsolutePath    string   `json:"absolutePath"`
	MIMEType        string   `json:"mimeType"`
	FileSizeBytes   int64    `json:"fileSizeBytes"`
	BatchHint       string   `json:"batchHint"`
	IndexedAudioID  *int64   `json:"indexedAudioId,omitempty"`
	IndexedAudioURL string   `json:"indexedAudioUrl,omitempty"`
}

func main() {
	var count int
	var promptBatchSize int
	var parallelism int

	flag.IntVar(&count, "count", defaultTrackCount, "number of tracks to generate")
	flag.IntVar(&promptBatchSize, "prompt-batch-size", defaultPromptBatchSize, "number of prompts to request from Gemini per batch")
	flag.IntVar(&parallelism, "parallel", defaultParallelism, "number of concurrent music generations")
	flag.Parse()

	if count <= 0 {
		log.Fatal("count must be greater than 0")
	}
	if promptBatchSize <= 0 {
		log.Fatal("prompt-batch-size must be greater than 0")
	}
	if parallelism <= 0 {
		log.Fatal("parallel must be greater than 0")
	}

	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}
	if cfg.GoogleCredentials == "" {
		log.Printf("GOOGLE_APPLICATION_CREDENTIALS is not set or could not be resolved")
	} else {
		log.Printf("using Google application credentials at %q", cfg.GoogleCredentials)
	}

	fileStore, err := storage.NewFileStore(cfg.AudioDir)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	repo, err := sqlite.NewAudioRepository(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	musicClient, err := vertexai.NewMusicClient(context.Background(), cfg.GoogleCloudProject, cfg.VertexLocation, cfg.LyriaModel)
	if err != nil {
		log.Fatal(err)
	}

	embeddingClient := gemini.NewClient(cfg.GeminiAPIKey, cfg.GeminiModel)
	service := usecase.NewServiceWithMusic(embeddingClient, musicClient, repo, fileStore)
	promptGenerator := gemini.NewPromptGenerator(cfg.GeminiAPIKey, "gemini-2.5-flash")
	runID := time.Now().UTC().Format("20060102T150405Z")
	manifestPath := filepath.Join(cfg.DataDir, fmt.Sprintf("lyria-batch-%s.json", runID))
	manifest := batchManifest{
		RunID:       runID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Requested:   count,
		AudioDir:    cfg.AudioDir,
		DBPath:      cfg.DBPath,
		Manifest:    manifestPath,
		PromptModel: promptGenerator.ModelName(),
		EmbedModel:  embeddingClient.ModelName(),
		MusicModel:  musicClient.ModelName(),
		Parallelism: parallelism,
		Tracks:      make([]batchManifestTrack, 0, count),
	}

	saved := 0
	attempts := 0
	for saved < count && attempts < maxGenerateAttempts {
		remaining := count - saved
		attemptBudget := maxGenerateAttempts - attempts
		batchSize := minInt(promptBatchSize, remaining, attemptBudget)
		if batchSize <= 0 {
			break
		}
		batchHint := coverageHintFor(saved / promptBatchSize)

		prompts, err := generatePromptBatch(promptGenerator, batchSize, batchHint)
		if err != nil {
			message := fmt.Sprintf("generate prompt batch failed after %d tracks: %v", saved, err)
			log.Print(message)
			manifest.Errors = append(manifest.Errors, message)
			break
		}

		generated := generateBatchInParallel(musicClient, prompts, parallelism)
		attempts += len(generated)

		for _, result := range generated {
			if result.err != nil {
				message := fmt.Sprintf("track %d failed for title=%q: %v", saved+1, result.prompt.Title, result.err)
				log.Print(message)
				manifest.Errors = append(manifest.Errors, message)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			clip, err := service.StoreGeneratedClip(ctx, buildOriginalFilename(saved+1, result.prompt.Title, result.clip.MIMEType), result.clip)
			cancel()
			if err != nil {
				if clip.Filename != "" {
					_ = os.Remove(fileStore.AudioPath(clip.Filename))
				}
				message := fmt.Sprintf("track %d failed while storing title=%q: %v", saved+1, result.prompt.Title, err)
				log.Print(message)
				manifest.Errors = append(manifest.Errors, message)
				continue
			}

			absolutePath := fileStore.AudioPath(clip.Filename)
			saved++
			manifest.Tracks = append(manifest.Tracks, batchManifestTrack{
				Index:           saved,
				Title:           result.prompt.Title,
				Genre:           result.prompt.Genre,
				Mood:            result.prompt.Mood,
				Tags:            append([]string(nil), result.prompt.Tags...),
				Prompt:          result.prompt.Prompt,
				NegativePrompt:  result.prompt.NegativePrompt,
				Filename:        filepath.Base(clip.Filename),
				RelativePath:    clip.Filename,
				AbsolutePath:    absolutePath,
				MIMEType:        clip.MIMEType,
				FileSizeBytes:   clip.FileSizeBytes,
				BatchHint:       batchHint,
				IndexedAudioID:  clip.IndexedAudioID,
				IndexedAudioURL: clip.IndexedAudioURL,
			})

			log.Printf("saved %d/%d: %s", saved, count, absolutePath)
		}
	}

	manifest.Saved = len(manifest.Tracks)
	if err := writeManifest(manifestPath, manifest); err != nil {
		log.Fatalf("write manifest: %v", err)
	}

	if manifest.Saved != count {
		log.Fatalf("generation incomplete: saved %d/%d tracks, manifest=%s", manifest.Saved, count, manifestPath)
	}

	log.Printf("completed: saved %d tracks to %s", manifest.Saved, cfg.AudioDir)
	log.Printf("manifest: %s", manifestPath)
}

func generatePromptBatch(generator *gemini.PromptGenerator, count int, batchHint string) ([]gemini.MusicPromptPlan, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		prompts, err := generator.GenerateBatch(ctx, count, batchHint)
		cancel()
		if err == nil {
			return prompts, nil
		}

		lastErr = err
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}

	return nil, fmt.Errorf("prompt generation retries exhausted: %w", lastErr)
}

type generatedTrackResult struct {
	prompt gemini.MusicPromptPlan
	clip   domain.GeneratedAudioSample
	err    error
}

func generateBatchInParallel(client *vertexai.Client, prompts []gemini.MusicPromptPlan, parallelism int) []generatedTrackResult {
	results := make([]generatedTrackResult, len(prompts))
	if len(prompts) == 0 {
		return results
	}

	workers := minInt(parallelism, len(prompts))
	indexCh := make(chan int)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				clip, err := generateTrackWithRetry(client, prompts[idx])
				results[idx] = generatedTrackResult{
					prompt: prompts[idx],
					clip:   clip,
					err:    err,
				}
			}
		}()
	}

	for idx := range prompts {
		indexCh <- idx
	}
	close(indexCh)
	wg.Wait()

	return results
}

func generateTrackWithRetry(client *vertexai.Client, prompt gemini.MusicPromptPlan) (domain.GeneratedAudioSample, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		output, err := client.GenerateMusic(ctx, domain.MusicGenerationRequest{
			Prompt:         prompt.Prompt,
			NegativePrompt: prompt.NegativePrompt,
			SampleCount:    1,
		})
		cancel()
		if err == nil {
			if len(output.Clips) == 0 {
				return domain.GeneratedAudioSample{}, fmt.Errorf("lyria returned no clips")
			}
			return output.Clips[0], nil
		}

		lastErr = err
		time.Sleep(time.Duration(attempt) * 3 * time.Second)
	}

	return domain.GeneratedAudioSample{}, fmt.Errorf("music generation retries exhausted: %w", lastErr)
}

func writeManifest(path string, manifest batchManifest) error {
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func coverageHintFor(batchIndex int) string {
	hints := []string{
		"Explore ambient, drone, healing, and deep-sleep textures with slow movement and rich spatial detail.",
		"Cover house, techno, electro, and breakbeat material with club-focused low end and strong rhythmic identity.",
		"Cover orchestral, chamber, fantasy, and cinematic score ideas with dynamic builds and dramatic contrast.",
		"Cover jazz, funk, soul-jazz, and lounge arrangements with expressive rhythm sections and live-player nuance.",
		"Cover rock, shoegaze, post-rock, and metallic instrumental ideas with guitar-driven energy and varied density.",
		"Cover acoustic, folk, Americana, bluegrass, and intimate handmade textures with organic room feel.",
		"Cover lo-fi hip-hop, boom bap, trap instrumentals, and smoky late-night beat tapes with strong grooves.",
		"Cover retro game, chiptune, synthwave, city pop, and vintage electronic nostalgia with memorable motifs.",
		"Cover Afro-Latin, reggae, dub, tropical, and world-fusion rhythms with percussion-forward arrangements.",
		"Cover experimental, glitch, industrial, modular, and avant-garde sound design with unusual timbres and structure.",
	}

	if len(hints) == 0 {
		return "Create clearly different instrumental prompts with broad coverage."
	}

	return hints[batchIndex%len(hints)]
}

func buildOriginalFilename(index int, title, mimeType string) string {
	ext := extensionForMIME(mimeType)
	slug := slugify(title)
	if slug == "" {
		slug = fmt.Sprintf("track-%03d", index)
	}
	return fmt.Sprintf("lyria-batch-%03d-%s%s", index, slug, ext)
}

func extensionForMIME(mimeType string) string {
	if mimeType != "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			return exts[0]
		}
	}

	switch mimeType {
	case "audio/wav", "audio/x-wav":
		return ".wav"
	default:
		return ".bin"
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len(value) > 40 {
		value = strings.Trim(value[:40], "-")
	}
	return value
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}

	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}
