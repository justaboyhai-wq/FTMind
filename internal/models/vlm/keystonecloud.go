package vlm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/justaboyhai-wq/keystone/internal/logger"
	"github.com/justaboyhai-wq/keystone/internal/models/utils"
	"github.com/google/uuid"
)

const keystoneCloudVLMPath = "/api/v1/chat/completions"

// KeystoneCloudVLM implements VLM via the KeystoneCloud API.
type KeystoneCloudVLM struct {
	modelName       string
	remoteModelName string
	modelID         string
	appID           string
	apiKey          string
	baseURL         string
	client          *http.Client
}

// NewKeystoneCloudVLM creates a KeystoneCloud-backed VLM instance.
func NewKeystoneCloudVLM(config *Config) (*KeystoneCloudVLM, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("KeystoneCloud VLM: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("KeystoneCloud VLM: AppSecret is required")
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if err := validateVLMBaseURL(baseURL); err != nil {
		return nil, err
	}
	remoteModelName := ""
	if config.Extra != nil {
		if v, ok := config.Extra["remote_model_name"]; ok {
			if vs, ok := v.(string); ok {
				remoteModelName = strings.TrimSpace(vs)
			}
		}
	}
	return &KeystoneCloudVLM{
		modelName:       config.ModelName,
		remoteModelName: remoteModelName,
		modelID:         config.ModelID,
		appID:           config.AppID,
		apiKey:          config.AppSecret,
		baseURL:         baseURL,
		client:          newVLMHTTPClient(vlmHTTPTimeout()),
	}, nil
}

type keystoneCloudVLMContentPart struct {
	Type     string                   `json:"type"`
	Text     string                   `json:"text,omitempty"`
	ImageURL *keystoneCloudVLMImageURL `json:"image_url,omitempty"`
}

type keystoneCloudVLMImageURL struct {
	URL string `json:"url"`
}

type keystoneCloudVLMMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type keystoneCloudVLMRequest struct {
	Model       string                   `json:"model"`
	Messages    []keystoneCloudVLMMessage `json:"messages"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	Stream      bool                     `json:"stream"`
}

type keystoneCloudVLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Predict sends images with a text prompt to the KeystoneCloud API.
func (v *KeystoneCloudVLM) Predict(ctx context.Context, imgBytesList [][]byte, prompt string) (string, error) {
	var parts []keystoneCloudVLMContentPart

	parts = append(parts, keystoneCloudVLMContentPart{
		Type: "text",
		Text: prompt,
	})

	for _, imgBytes := range imgBytesList {
		if len(imgBytes) > 0 {
			mimeType := detectImageMIME(imgBytes)
			b64 := base64.StdEncoding.EncodeToString(imgBytes)
			dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
			parts = append(parts, keystoneCloudVLMContentPart{
				Type: "image_url",
				ImageURL: &keystoneCloudVLMImageURL{
					URL: dataURI,
				},
			})
		}
	}

	reqBody := keystoneCloudVLMRequest{
		Model: v.effectiveModelName(),
		Messages: []keystoneCloudVLMMessage{
			{
				Role:    "user",
				Content: parts,
			},
		},
		MaxTokens:   defaultMaxToks,
		Temperature: float64(defaultTemp),
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("keystonecloud VLM: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(v.appID, v.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.baseURL+keystoneCloudVLMPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("keystonecloud VLM: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, hv := range headers {
		req.Header.Set(k, hv)
	}

	totalImageSize := 0
	for _, img := range imgBytesList {
		totalImageSize += len(img)
	}
	logger.Infof(ctx, "[VLM] Calling KeystoneCloud API, model=%s, baseURL=%s, numImages=%d, totalImageSize=%d",
		v.effectiveModelName(), v.baseURL, len(imgBytesList), totalImageSize)

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("keystonecloud VLM: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("keystonecloud VLM: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keystonecloud VLM: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var vlmResp keystoneCloudVLMResponse
	if err := json.Unmarshal(respBytes, &vlmResp); err != nil {
		return "", fmt.Errorf("keystonecloud VLM: unmarshal: %w", err)
	}
	if len(vlmResp.Choices) == 0 {
		return "", fmt.Errorf("keystonecloud VLM: no choices in response")
	}

	content := vlmResp.Choices[0].Message.Content
	logger.Infof(ctx, "[VLM] KeystoneCloud response received, len=%d", len(content))
	return content, nil
}

func (v *KeystoneCloudVLM) effectiveModelName() string {
	if v.remoteModelName != "" {
		return v.remoteModelName
	}
	return v.modelName
}

func (v *KeystoneCloudVLM) GetModelName() string { return v.modelName }
func (v *KeystoneCloudVLM) GetModelID() string   { return v.modelID }
