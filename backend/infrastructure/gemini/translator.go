package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type generateContentRequest struct {
	Contents          []generateContent `json:"contents"`
	SystemInstruction *generateContent  `json:"systemInstruction,omitempty"`
}

type generateContent struct {
	Parts []generatePart `json:"parts"`
}

type generatePart struct {
	Text string `json:"text,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// Translator は Gemini API を使って日本語プロンプトを英語へ翻訳します。
type Translator struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewTranslator は Gemini 翻訳クライアントを生成します。
func NewTranslator(apiKey, model string) *Translator {
	return &Translator{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ModelName は設定済みの翻訳モデル名を返します。
func (t *Translator) ModelName() string {
	return t.model
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
					Text: "Translate the user's music-generation prompt into concise natural English for a music model. Preserve mood, instrumentation, rhythm, and scene details. Return only the English translation.",
				},
			},
		},
		Contents: []generateContent{
			{
				Parts: []generatePart{{Text: text}},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", t.model, t.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		var apiErr errorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var generateResp generateContentResponse
	if err := json.Unmarshal(respBody, &generateResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(generateResp.Candidates) == 0 || len(generateResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty translation returned by Gemini")
	}

	translated := strings.TrimSpace(generateResp.Candidates[0].Content.Parts[0].Text)
	if translated == "" {
		return "", fmt.Errorf("empty translation returned by Gemini")
	}

	return translated, nil
}
