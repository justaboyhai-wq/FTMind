package web_search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/justaboyhai-wq/keystone/internal/logger"
	"github.com/justaboyhai-wq/keystone/internal/types"
	"github.com/justaboyhai-wq/keystone/internal/types/interfaces"
)

const (
	// AgentPlan must use its dedicated gateway so requests are billed to the
	// customer's AgentPlan subscription rather than the standard Ark account.
	defaultArkAgentPlanResponsesURL = "https://ark.cn-beijing.volces.com/api/plan/v3/responses"
	// AgentPlan's model catalogue exposes this model name directly. Keep the
	// search adapter aligned with the defaults that Keystone creates for an
	// AgentPlan account instead of using a standard-Ark date-suffixed alias.
	defaultArkAgentPlanModel = "doubao-seed-2.0-lite"
	defaultArkTimeout        = 45 * time.Second
	maxArkResults            = 10
)

// ArkProvider adapts Ark's built-in Web Search tool to Keystone's normal
// provider contract. The model performs the web search; this provider asks it
// to return a small, source-preserving JSON result set for the existing agent
// toolchain and citation UI.
type ArkProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

func NewArkProvider(params types.WebSearchProviderParameters) (interfaces.WebSearchProvider, error) {
	if params.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Ark AgentPlan provider")
	}
	client, err := NewSearchHTTPClient(defaultArkTimeout, params.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &ArkProvider{client: client, baseURL: defaultArkAgentPlanResponsesURL, apiKey: params.APIKey}, nil
}

func (p *ArkProvider) Name() string { return "ark" }

func (p *ArkProvider) Search(ctx context.Context, query string, maxResults int, _ bool) ([]*types.WebSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > maxArkResults {
		maxResults = maxArkResults
	}

	prompt := fmt.Sprintf(`Search the public web for: %q. You must use the web_search tool. Return ONLY a JSON array with at most %d objects. Each object must contain title, url, and snippet. Keep snippet factual and concise.`, query, maxResults)
	payload := map[string]any{
		"model": defaultArkAgentPlanModel,
		"input": prompt,
		"tools": []map[string]string{{"type": "web_search"}},
		"store": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Ark web search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Ark web search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultUserAgentHeader)

	logger.Infof(ctx, "[WebSearch][Ark] query=%q maxResults=%d", query, maxResults)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Ark AgentPlan: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read Ark response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Ark AgentPlan API returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	results, err := parseArkSearchResponse(responseBody)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Ark AgentPlan returned no web search results; verify that the Ark Web Search plugin is enabled for this API key")
	}
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	logger.Infof(ctx, "[WebSearch][Ark] returned %d results", len(results))
	return results, nil
}

type arkResponsesEnvelope struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

type arkSearchItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func parseArkSearchResponse(body []byte) ([]*types.WebSearchResult, error) {
	var envelope arkResponsesEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode Ark response: %w", err)
	}
	var texts []string
	for _, output := range envelope.Output {
		if output.Type != "message" {
			continue
		}
		for _, content := range output.Content {
			if content.Text != "" {
				texts = append(texts, content.Text)
			}
		}
	}
	if len(texts) == 0 {
		return nil, fmt.Errorf("Ark response did not contain a text result")
	}
	text := strings.TrimSpace(strings.Join(texts, "\n"))
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	var items []arkSearchItem
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		return nil, fmt.Errorf("Ark response was not a search result array: %w", err)
	}
	results := make([]*types.WebSearchResult, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		results = append(results, &types.WebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Snippet, Source: "ark"})
	}
	return results, nil
}
