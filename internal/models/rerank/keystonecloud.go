package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/justaboyhai-wq/keystone/internal/models/utils"
	"github.com/google/uuid"
)

const keystoneCloudRerankPath = "/api/v1/rerank"

// KeystoneCloudReranker 实现 rerank.Reranker 接口，对接 KeystoneCloud /api/v1/rerank
type KeystoneCloudReranker struct {
	modelName       string
	remoteModelName string
	modelID         string
	appID           string
	apiKey          string
	baseURL         string
	client          *http.Client
}

// NewKeystoneCloudReranker 构造 KeystoneCloudReranker
func NewKeystoneCloudReranker(config *RerankerConfig) (*KeystoneCloudReranker, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("KeystoneCloud reranker: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("KeystoneCloud reranker: AppSecret is required")
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if err := validateRerankBaseURL(baseURL); err != nil {
		return nil, err
	}
	remoteModelName := ""
	if config.ExtraConfig != nil {
		remoteModelName = strings.TrimSpace(config.ExtraConfig["remote_model_name"])
	}
	return &KeystoneCloudReranker{
		modelName:       config.ModelName,
		remoteModelName: remoteModelName,
		modelID:         config.ModelID,
		appID:           config.AppID,
		apiKey:          config.AppSecret,
		baseURL:         baseURL,
		client:          newRerankHTTPClient(60 * time.Second),
	}, nil
}

type keystoneCloudRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type keystoneCloudRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document"`
	} `json:"results"`
}

func (r *KeystoneCloudReranker) Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error) {
	reqBody := keystoneCloudRerankRequest{
		Model:     r.effectiveModelName(),
		Query:     query,
		Documents: documents,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("keystonecloud reranker: marshal: %w", err)
	}

	requestID := uuid.New().String()
	headers := utils.Sign(r.appID, r.apiKey, requestID, string(bodyBytes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+keystoneCloudRerankPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("keystonecloud reranker: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keystonecloud reranker: do request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("keystonecloud reranker: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keystonecloud reranker: status %d: %s", resp.StatusCode, string(respBytes))
	}

	var rerankResp keystoneCloudRerankResponse
	if err := json.Unmarshal(respBytes, &rerankResp); err != nil {
		return nil, fmt.Errorf("keystonecloud reranker: unmarshal: %w", err)
	}

	results := make([]RankResult, 0, len(rerankResp.Results))
	for _, item := range rerankResp.Results {
		results = append(results, RankResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
			Document:       DocumentInfo{Text: item.Document.Text},
		})
	}
	return results, nil
}

func (r *KeystoneCloudReranker) effectiveModelName() string {
	if r.remoteModelName != "" {
		return r.remoteModelName
	}
	return r.modelName
}

func (r *KeystoneCloudReranker) GetModelName() string { return r.modelName }
func (r *KeystoneCloudReranker) GetModelID() string   { return r.modelID }
