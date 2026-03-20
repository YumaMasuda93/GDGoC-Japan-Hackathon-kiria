package gemini

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Translator は Gemini API を使って日本語プロンプトを英語へ翻訳します。
type Translator struct {
	generator *contentGenerator
}

// NewTranslator は Gemini 翻訳クライアントを生成します。
func NewTranslator(apiKey, model string) *Translator {
	return &Translator{
		generator: newContentGenerator(apiKey, model, 60*time.Second),
	}
}

// ModelName は設定済みの翻訳モデル名を返します。
func (t *Translator) ModelName() string {
	return t.generator.ModelName()
}

// TranslateToEnglish は日本語中心の音楽プロンプトを英語へ翻訳します。
func (t *Translator) TranslateToEnglish(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}

	payload := generateContentRequest{
		SystemInstruction: &generateContent{
			Parts: []generatePart{
				{
					Text: "Translate the user's music-generation prompt into concise natural English for a music model. Preserve mood, instrumentation, rhythm, and scene details, but abstract away any lyric-like wording, quoted phrases, artist references, song-title references, or other uniquely identifying expressions. Do not preserve recognizable lines verbatim. Return only the English translation.",
				},
			},
		},
		Contents: []generateContent{
			{
				Parts: []generatePart{{Text: text}},
			},
		},
	}

	translated, err := t.generator.GenerateText(ctx, payload)
	if err != nil {
		return "", fmt.Errorf("generate translation: %w", err)
	}
	return translated, nil
}
