package vertexai

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

	"kiria/backend/domain"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

type predictRequest struct {
	Instances  []predictInstance `json:"instances"`
	Parameters predictParameters `json:"parameters,omitempty"`
}

type predictInstance struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Seed           *int64 `json:"seed,omitempty"`
}

type predictParameters struct {
	SampleCount int `json:"sample_count,omitempty"`
}

type predictResponse struct {
	Predictions []struct {
		AudioContent       string `json:"audioContent"`
		BytesBase64Encoded string `json:"bytesBase64Encoded"`
		Audio              string `json:"audio"`
		MIMEType           string `json:"mimeType"`
	} `json:"predictions"`
	Model        string `json:"model"`
	ModelDisplay string `json:"modelDisplayName"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client は Vertex AI の Lyria モデルを呼び出します。
type Client struct {
	projectID   string
	location    string
	model       string
	tokenSource oauth2.TokenSource
	httpClient  *http.Client
}

// NewMusicClient は Application Default Credentials を使う Lyria クライアントを作成します。
func NewMusicClient(ctx context.Context, projectID, location, model string) (*Client, error) {
	creds, err := google.FindDefaultCredentials(ctx, cloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("find application default credentials: %w", err)
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		projectID = strings.TrimSpace(creds.ProjectID)
	}
	if projectID == "" {
		return nil, errors.New("GOOGLE_CLOUD_PROJECT is required for Vertex AI music generation")
	}

	location = strings.TrimSpace(location)
	if location == "" {
		location = "us-central1"
	}

	model = strings.TrimSpace(model)
	if model == "" {
		model = "lyria-002"
	}

	return &Client{
		projectID:   projectID,
		location:    location,
		model:       model,
		tokenSource: creds.TokenSource,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}, nil
}

// ModelName は設定済みの Lyria モデル名を返します。
func (c *Client) ModelName() string {
	return c.model
}

// GenerateMusic は Vertex AI Lyria API で音楽クリップを生成します。
func (c *Client) GenerateMusic(ctx context.Context, req domain.MusicGenerationRequest) (domain.MusicGenerationOutput, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return domain.MusicGenerationOutput{}, errors.New("prompt is required")
	}
	if req.Seed != nil && req.SampleCount > 0 {
		return domain.MusicGenerationOutput{}, errors.New("seed cannot be used with sampleCount")
	}
	if req.SampleCount < 0 || req.SampleCount > 4 {
		return domain.MusicGenerationOutput{}, errors.New("sampleCount must be between 1 and 4 when provided")
	}

	payload := predictRequest{
		Instances: []predictInstance{
			{
				Prompt:         prompt,
				NegativePrompt: strings.TrimSpace(req.NegativePrompt),
				Seed:           req.Seed,
			},
		},
	}
	if req.SampleCount > 0 {
		payload.Parameters.SampleCount = req.SampleCount
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return domain.MusicGenerationOutput{}, fmt.Errorf("marshal request: %w", err)
	}

	token, err := c.tokenSource.Token()
	if err != nil {
		return domain.MusicGenerationOutput{}, fmt.Errorf("fetch access token: %w", err)
	}

	url := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		c.location,
		c.projectID,
		c.location,
		c.model,
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return domain.MusicGenerationOutput{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.MusicGenerationOutput{}, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.MusicGenerationOutput{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		var apiErr errorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return domain.MusicGenerationOutput{}, fmt.Errorf("status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return domain.MusicGenerationOutput{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var predictResp predictResponse
	if err := json.Unmarshal(respBody, &predictResp); err != nil {
		return domain.MusicGenerationOutput{}, fmt.Errorf("decode response: %w", err)
	}
	if len(predictResp.Predictions) == 0 {
		return domain.MusicGenerationOutput{}, errors.New("empty predictions returned by Lyria")
	}

	result := domain.MusicGenerationOutput{
		Model:        predictResp.Model,
		ModelDisplay: predictResp.ModelDisplay,
		Clips:        make([]domain.GeneratedAudioSample, 0, len(predictResp.Predictions)),
	}

	for _, prediction := range predictResp.Predictions {
		var encodedAudio string
		if prediction.AudioContent != "" {
			encodedAudio = prediction.AudioContent
		} else if prediction.BytesBase64Encoded != "" {
			encodedAudio = prediction.BytesBase64Encoded
		} else if prediction.Audio != "" {
			encodedAudio = prediction.Audio
		} else {
			return domain.MusicGenerationOutput{}, errors.New("missing audio content in prediction")
		}

		audioData, err := base64.StdEncoding.DecodeString(encodedAudio)
		if err != nil {
			return domain.MusicGenerationOutput{}, fmt.Errorf("decode audio content: %w", err)
		}

		mimeType := prediction.MIMEType
		if mimeType == "" {
			mimeType = "audio/wav"
		}

		result.Clips = append(result.Clips, domain.GeneratedAudioSample{
			MIMEType:  mimeType,
			AudioData: audioData,
		})
	}

	return result, nil
}
