package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type embedRequest struct {
	Model    string  `json:"model,omitempty"`
	Content  content `json:"content"`
	TaskType string  `json:"taskType,omitempty"`
	Title    string  `json:"title,omitempty"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type inlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type embedResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client calls the Gemini embeddings API.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient constructs a Gemini API client.
func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// ModelName returns the configured embedding model.
func (c *Client) ModelName() string {
	return c.model
}

// EmbedText generates a query embedding for online retrieval.
func (c *Client) EmbedText(ctx context.Context, text string) ([]float64, error) {
	req := embedRequest{
		Model:    modelResourceName(c.model),
		TaskType: "RETRIEVAL_QUERY",
		Content: content{
			Parts: []part{{Text: text}},
		},
	}
	return c.embed(ctx, req)
}

// EmbedAudio generates an embedding for an audio document that will be indexed.
func (c *Client) EmbedAudio(ctx context.Context, mimeType string, audioData []byte, title string) ([]float64, error) {
	req := embedRequest{
		Model:    modelResourceName(c.model),
		TaskType: "RETRIEVAL_DOCUMENT",
		Title:    title,
		Content: content{
			Parts: []part{
				{
					InlineData: &inlineData{
						MIMEType: mimeType,
						Data:     base64.StdEncoding.EncodeToString(audioData),
					},
				},
			},
		},
	}
	return c.embed(ctx, req)
}

func (c *Client) embed(ctx context.Context, payload embedRequest) ([]float64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s", c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		var apiErr errorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp embedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(embedResp.Embedding.Values) == 0 {
		return nil, errors.New("empty embedding returned by Gemini")
	}

	return embedResp.Embedding.Values, nil
}

func modelResourceName(model string) string {
	if strings.HasPrefix(model, "models/") {
		return model
	}
	return "models/" + model
}
