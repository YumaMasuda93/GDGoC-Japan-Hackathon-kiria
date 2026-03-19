package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const defaultPromptModel = "gemini-2.5-flash"

// MusicPromptPlan は Lyria に渡す英語プロンプト 1 件分です。
type MusicPromptPlan struct {
	Title          string   `json:"title"`
	Prompt         string   `json:"prompt"`
	NegativePrompt string   `json:"negativePrompt"`
	Genre          string   `json:"genre"`
	Mood           string   `json:"mood"`
	Tags           []string `json:"tags,omitempty"`
}

type musicPromptEnvelope struct {
	Prompts []MusicPromptPlan `json:"prompts"`
}

// PromptGenerator は Gemini API で Lyria 向けプロンプト群を生成します。
type PromptGenerator struct {
	generator *contentGenerator
}

// NewPromptGenerator は `gemini-2.5-flash` を既定とするプロンプト生成クライアントを返します。
func NewPromptGenerator(apiKey, model string) *PromptGenerator {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultPromptModel
	}

	return &PromptGenerator{
		generator: newContentGenerator(apiKey, model, 90*time.Second),
	}
}

// ModelName は設定済みの Gemini モデル名を返します。
func (g *PromptGenerator) ModelName() string {
	return g.generator.ModelName()
}

// GenerateBatch は多様性ヒントに沿って Lyria 用プロンプトをまとめて生成します。
func (g *PromptGenerator) GenerateBatch(ctx context.Context, count int, diversityHint string) ([]MusicPromptPlan, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be greater than 0")
	}

	temperature := 1.1
	payload := generateContentRequest{
		SystemInstruction: &generateContent{
			Parts: []generatePart{
				{
					Text: "You create high-quality English prompts for Google Lyria music generation. Return only JSON matching the schema. Make every item clearly distinct in genre, tempo, rhythm, arrangement, instrumentation, production style, and emotional tone. Avoid artist names, copyrighted song titles, and references to existing melodies. Each prompt must be concise but vivid, and must describe the music itself rather than a request to the model.",
				},
			},
		},
		Contents: []generateContent{
			{
				Parts: []generatePart{
					{
						Text: fmt.Sprintf(
							"Generate %d instrumental music prompt plans for Lyria. Focus on broad stylistic coverage.\nDiversity hint: %s\nRequirements:\n- English only\n- 1 to 3 sentences per prompt\n- Mention instrumentation, groove or tempo feel, production texture, and mood\n- negativePrompt must be a short comma-separated list\n- Prefer no vocals, no spoken word, and no crowd noise unless the concept truly requires ambience\n- title should be short and specific\n- genre and mood should be compact labels\n- tags should contain 3 to 6 short descriptors",
							count,
							strings.TrimSpace(diversityHint),
						),
					},
				},
			},
		},
		GenerationConfig: &generateContentConfig{
			ResponseMIMEType:   "application/json",
			ResponseJSONSchema: promptResponseSchema(count),
			Temperature:        &temperature,
		},
	}

	text, err := g.generator.GenerateText(ctx, payload)
	if err != nil {
		return nil, err
	}

	var envelope musicPromptEnvelope
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, fmt.Errorf("decode prompt batch: %w", err)
	}

	plans := normalizePromptPlans(envelope.Prompts)
	if len(plans) != count {
		return nil, fmt.Errorf("expected %d prompts, got %d", count, len(plans))
	}

	return plans, nil
}

func promptResponseSchema(count int) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"prompts": map[string]any{
				"type":     "array",
				"minItems": count,
				"maxItems": count,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "A short unique track title.",
						},
						"prompt": map[string]any{
							"type":        "string",
							"description": "An English Lyria-ready prompt describing arrangement, mood, instrumentation, rhythm, and sonic texture.",
						},
						"negativePrompt": map[string]any{
							"type":        "string",
							"description": "A short comma-separated list of unwanted traits.",
						},
						"genre": map[string]any{
							"type":        "string",
							"description": "A compact genre label.",
						},
						"mood": map[string]any{
							"type":        "string",
							"description": "A compact mood label.",
						},
						"tags": map[string]any{
							"type":     "array",
							"minItems": 3,
							"maxItems": 6,
							"items": map[string]any{
								"type": "string",
							},
							"description": "Short descriptors for search and cataloging.",
						},
					},
					"required": []string{"title", "prompt", "negativePrompt", "genre", "mood", "tags"},
				},
			},
		},
		"required": []string{"prompts"},
	}
}

func normalizePromptPlans(plans []MusicPromptPlan) []MusicPromptPlan {
	normalized := make([]MusicPromptPlan, 0, len(plans))
	for _, plan := range plans {
		plan.Title = strings.TrimSpace(plan.Title)
		plan.Prompt = strings.TrimSpace(plan.Prompt)
		plan.NegativePrompt = strings.TrimSpace(plan.NegativePrompt)
		plan.Genre = strings.TrimSpace(plan.Genre)
		plan.Mood = strings.TrimSpace(plan.Mood)

		tags := make([]string, 0, len(plan.Tags))
		for _, tag := range plan.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			tags = append(tags, tag)
		}
		plan.Tags = tags

		if plan.Title == "" || plan.Prompt == "" || plan.NegativePrompt == "" || plan.Genre == "" || plan.Mood == "" || len(plan.Tags) == 0 {
			continue
		}
		normalized = append(normalized, plan)
	}

	return normalized
}
