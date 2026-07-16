package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/justaboyhai-wq/keystone/internal/models/provider"
	"github.com/justaboyhai-wq/keystone/internal/models/utils"
	"github.com/google/uuid"
)

const keystoneCloudEmbedPath = "/api/v1/embeddings"

// KeystoneCloudEmbedder 实现 embedding.Embedder 接口，对接 KeystoneCloud /api/v1/embeddings
type KeystoneCloudEmbedder struct {
	modelName                 string
	remoteModelName           string
	modelID                   string
	appID                     string
	apiKey                    string
	baseURL                   string
	dimensions                int
	supportsDimensionOverride bool
	client                    *http.Client
	EmbedderPooler
}

// NewKeystoneCloudEmbedder 构造 KeystoneCloudEmbedder
func NewKeystoneCloudEmbedder(config Config) (*KeystoneCloudEmbedder, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("KeystoneCloud embedder: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("KeystoneCloud embedder: AppSecret is required")
	}
	remoteModelName := ""
	if config.ExtraConfig != nil {
		remoteModelName = strings.TrimSpace(config.ExtraConfig["remote_model_name"])
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = provider.KeystoneCloudBaseURL
	}
	if err := validateEmbeddingBaseURL(baseURL); err != nil {
		return nil, err
	}
	return &KeystoneCloudEmbedder{
		modelName:                 config.ModelName,
		remoteModelName:           remoteModelName,
		modelID:                   config.ModelID,
		appID:                     config.AppID,
		apiKey:                    config.AppSecret,
		baseURL:                   baseURL,
		dimensions:                config.Dimensions,
		supportsDimensionOverride: config.SupportsDimensionOverride,
		client:                    newEmbeddingHTTPClient(60 * time.Second),
	}, nil
}

type keystoneCloudEmbedRequest struct {
	Model                string   `json:"model"`
	Input                []string `json:"input"`
	Dimensions           int      `json:"dimensions,omitempty"`
	TruncatePromptTokens int      `json:"truncate_prompt_tokens,omitempty"`
}

type keystoneCloudEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *KeystoneCloudEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := e.BatchEmbed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("keystonecloud embedder: empty response")
	}
	return results[0], nil
}

func (e *KeystoneCloudEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := keystoneCloudEmbedRequest{Model: e.effectiveModelName(), Input: texts}
	if e.supportsDimensionOverride && e.dimensions > 0 {
		reqBody.Dimensions = e.dimensions
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("keystonecloud embedder: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(e.appID, e.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+keystoneCloudEmbedPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("keystonecloud embedder: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keystonecloud embedder: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("keystonecloud embedder: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keystonecloud embedder: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var embedResp keystoneCloudEmbedResponse
	if err := json.Unmarshal(respBytes, &embedResp); err != nil {
		return nil, fmt.Errorf("keystonecloud embedder: unmarshal: %w", err)
	}

	result := make([][]float32, len(texts))
	for _, item := range embedResp.Data {
		if item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	return result, nil
}

func (e *KeystoneCloudEmbedder) BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error) {
	return e.BatchEmbed(ctx, texts)
}

func (e *KeystoneCloudEmbedder) SetSupportsDimensionOverride(supported bool) {
	e.supportsDimensionOverride = supported
}

func (e *KeystoneCloudEmbedder) effectiveModelName() string {
	if e.remoteModelName != "" {
		return e.remoteModelName
	}
	return e.modelName
}

func (e *KeystoneCloudEmbedder) GetModelName() string { return e.modelName }
func (e *KeystoneCloudEmbedder) GetModelID() string   { return e.modelID }
func (e *KeystoneCloudEmbedder) GetDimensions() int   { return e.dimensions }
