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
	Contents          []generateContent      `json:"contents"`
	SystemInstruction *generateContent       `json:"systemInstruction,omitempty"`
	GenerationConfig  *generateContentConfig `json:"generationConfig,omitempty"`
}

type generateContent struct {
	Parts []generatePart `json:"parts"`
}

type generatePart struct {
	Text string `json:"text,omitempty"`
}

type generateContentConfig struct {
	ResponseMIMEType   string         `json:"responseMimeType,omitempty"`
	ResponseJSONSchema map[string]any `json:"responseJsonSchema,omitempty"`
	Temperature        *float64       `json:"temperature,omitempty"`
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

type contentGenerator struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func newContentGenerator(apiKey, model string, timeout time.Duration) *contentGenerator {
	return &contentGenerator{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (g *contentGenerator) ModelName() string {
	return g.model
}

func (g *contentGenerator) GenerateText(ctx context.Context, payload generateContentRequest) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
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
		return "", fmt.Errorf("empty response returned by Gemini")
	}

	text := strings.TrimSpace(generateResp.Candidates[0].Content.Parts[0].Text)
	if text == "" {
		return "", fmt.Errorf("empty response returned by Gemini")
	}

	return text, nil
}
