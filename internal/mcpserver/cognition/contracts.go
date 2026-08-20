// Package cognition defines the stable MCP contract for external-agent memory
// access. The existing FTMind MCP client/server integrations remain unchanged;
// this contract is an additive bridge for MemoryProxy and future connectors.
package cognition

import (
	"errors"
)

const (
	ToolMemoryGetContext       = "memory_get_context"
	ToolMemorySearch           = "memory_search"
	ToolMemoryCaptureTurn      = "memory_capture_turn"
	ToolMemoryConfirmCandidate = "memory_confirm_candidate"
	ToolKnowledgeSearch        = "knowledge_search"
	ToolWikiGetPage            = "wiki_get_page"
	ToolDocumentRead           = "document_read"
	ToolContextAssemble        = "context_assemble"
)

type Request struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
	TraceID   string         `json:"trace_id,omitempty"`
}
type Response struct {
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

func ValidateRequest(r Request) error {
	if _, ok := toolPolicies[r.Tool]; !ok {
		return errors.New("unsupported cognition tool")
	}
	return nil
}
