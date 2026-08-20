package cognition

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type disabledMemoryGateway struct{}

func (disabledMemoryGateway) Invoke(context.Context, string, types.BindingContext, string, map[string]any, string) (any, error) {
	return nil, errors.New("MemoryCore gateway is not configured")
}

// NewMemoryGatewayFromEnvironment keeps the original FTMind deployment usable
// without MemoryCore. Once any MemoryCore endpoint is configured, credentials
// and transport policy become mandatory and startup fails closed.
func NewMemoryGatewayFromEnvironment() (MemoryGateway, error) {
	baseURL := strings.TrimSpace(os.Getenv("FMIND_MEMORY_CORE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("FMIND_MEMORY_CORE_API_KEY"))
	serviceID := strings.TrimSpace(os.Getenv("FMIND_MEMORY_CORE_SERVICE_ID"))
	// Service ID has a useful default for an enabled gateway, but by itself it
	// must not opt a normal FTMind-only deployment into the MemoryCore client.
	// Compose always supplies FMIND_MEMORY_CORE_SERVICE_ID, even when the
	// optional MemoryCore profile is disabled.
	if baseURL == "" && apiKey == "" {
		return disabledMemoryGateway{}, nil
	}
	allowInsecure, err := strconv.ParseBool(defaultString(os.Getenv("FMIND_MEMORY_CORE_ALLOW_INSECURE_HTTP"), "false"))
	if err != nil {
		return nil, errors.New("FMIND_MEMORY_CORE_ALLOW_INSECURE_HTTP must be a boolean")
	}
	timeout := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("FMIND_MEMORY_CORE_TIMEOUT")); raw != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil || timeout <= 0 {
			return nil, errors.New("FMIND_MEMORY_CORE_TIMEOUT must be a positive duration")
		}
	}
	return NewHTTPMemoryGateway(MemoryGatewayConfig{
		BaseURL:           baseURL,
		APIKey:            apiKey,
		ServiceID:         serviceID,
		Timeout:           timeout,
		AllowInsecureHTTP: allowInsecure,
		InsecureHosts:     splitNonEmpty(os.Getenv("FMIND_MEMORY_CORE_INSECURE_HOSTS")),
	})
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func NewBindingTokenVerifier(service interfaces.AgentBindingService) BindingTokenVerifier {
	return service
}

func NewKnowledgeSearcher(service interfaces.SessionService) KnowledgeSearcher {
	return service
}

func NewWikiReader(service interfaces.WikiPageService) WikiReader {
	return service
}

func NewDocumentReader(service interfaces.KnowledgeService) DocumentReader {
	return service
}

func NewDocumentChunkReader(service interfaces.ChunkService) DocumentChunkReader {
	return service
}

func NewToolExecutor(executor *DefaultExecutor) ToolExecutor {
	return executor
}
