package cognition

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

const maxMemoryGatewayResponseBytes = 4 << 20

type MemoryGatewayConfig struct {
	BaseURL           string
	APIKey            string
	ServiceID         string
	Timeout           time.Duration
	AllowInsecureHTTP bool
	InsecureHosts     []string
}

type HTTPMemoryGateway struct {
	baseURL   *url.URL
	apiKey    string
	serviceID string
	client    *http.Client
}

func NewHTTPMemoryGateway(config MemoryGatewayConfig) (*HTTPMemoryGateway, error) {
	baseURL, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("memory gateway base URL is invalid")
	}
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.ServiceID) == "" {
		return nil, errors.New("memory gateway credentials are required")
	}
	if err := validateMemoryGatewayURL(baseURL, config); err != nil {
		return nil, err
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPMemoryGateway{
		baseURL: baseURL, apiKey: config.APIKey, serviceID: config.ServiceID,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func validateMemoryGatewayURL(baseURL *url.URL, config MemoryGatewayConfig) error {
	if strings.EqualFold(baseURL.Scheme, "https") {
		return nil
	}
	if !strings.EqualFold(baseURL.Scheme, "http") {
		return errors.New("memory gateway requires HTTPS")
	}
	host := strings.ToLower(baseURL.Hostname())
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	if !config.AllowInsecureHTTP {
		return errors.New("memory gateway requires HTTPS")
	}
	for _, allowed := range config.InsecureHosts {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return nil
		}
	}
	return errors.New("memory gateway HTTP host is not explicitly allowed")
}

func (g *HTTPMemoryGateway) Invoke(ctx context.Context, tool string, binding types.BindingContext, bindingToken string, arguments map[string]any, traceID string) (any, error) {
	if g == nil || g.client == nil || strings.TrimSpace(bindingToken) == "" || binding.BindingID == "" || binding.TeamID == "" || binding.UserID == "" || binding.AgentID == "" || binding.PolicyVersion == 0 {
		return nil, errors.New("memory gateway authority is incomplete")
	}
	path, ok := memoryGatewayPaths[tool]
	if !ok {
		return nil, fmt.Errorf("%w: unsupported memory gateway tool", ErrInvalidRequest)
	}
	body := cloneArguments(arguments)
	for _, field := range []string{"tenant_id", "team_id", "department_id", "workspace_id", "project_id", "user_id", "agent_id", "task_id", "binding_id", "policy_version", "binding_token"} {
		delete(body, field)
	}
	externalSession, _ := body["external_session_id"].(string)
	delete(body, "external_session_id")
	sessionID := memorySessionID(binding, externalSession)
	if tool == ToolMemoryCaptureTurn {
		body["session_id"] = sessionID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: arguments are not JSON serializable", ErrInvalidRequest)
	}
	endpoint := g.baseURL.ResolveReference(&url.URL{Path: strings.TrimRight(g.baseURL.Path, "/") + path})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, errors.New("memory gateway request could not be created")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+g.apiKey)
	request.Header.Set("X-TDAI-Service-ID", g.serviceID)
	request.Header.Set("X-FMind-Binding-Token", bindingToken)
	request.Header.Set("X-TDAI-Team-ID", binding.TeamID)
	request.Header.Set("X-TDAI-User-ID", binding.UserID)
	request.Header.Set("X-TDAI-Agent-ID", binding.AgentID)
	request.Header.Set("X-TDAI-Session-ID", sessionID)
	if binding.TaskID != "" {
		request.Header.Set("X-TDAI-Task-ID", binding.TaskID)
	}
	if traceID != "" {
		request.Header.Set("X-Request-ID", traceID)
	}
	response, err := g.client.Do(request)
	if err != nil {
		return nil, errors.New("memory gateway request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
		return nil, errors.New("memory gateway rejected the request")
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxMemoryGatewayResponseBytes+1))
	decoder.UseNumber()
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := decoder.Decode(&envelope); err != nil || envelope.Code != 0 || len(envelope.Data) == 0 {
		return nil, errors.New("memory gateway returned an invalid response")
	}
	var result any
	resultDecoder := json.NewDecoder(bytes.NewReader(envelope.Data))
	resultDecoder.UseNumber()
	if err := resultDecoder.Decode(&result); err != nil {
		return nil, errors.New("memory gateway returned an invalid payload")
	}
	return result, nil
}

var memoryGatewayPaths = map[string]string{
	ToolMemoryCaptureTurn:      "/v3/conversation/add",
	ToolMemorySearch:           "/v3/atomic/search",
	ToolMemoryGetContext:       "/v3/cognition/context",
	ToolMemoryConfirmCandidate: "/v3/cognition/candidate/confirm",
}

func memorySessionID(binding types.BindingContext, externalSession string) string {
	externalSession = strings.TrimSpace(externalSession)
	if externalSession == "" {
		externalSession = "default"
	}
	digest := sha256.Sum256([]byte(externalSession))
	return binding.BindingID + ":p" + strconv.FormatUint(binding.PolicyVersion, 10) + ":" + hex.EncodeToString(digest[:8])
}
