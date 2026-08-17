// Package cognition defines the stable MCP contract for external-agent memory
// access. The existing FMind MCP client/server integrations remain unchanged;
// this contract is an additive bridge for MemoryProxy and future connectors.
package cognition

import (
	"errors"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

const (
	ToolMemoryGetContext      = "memory_get_context"
	ToolMemorySearch          = "memory_search"
	ToolMemoryCaptureTurn     = "memory_capture_turn"
	ToolMemorySubmitCandidate = "memory_submit_candidate"
	ToolKnowledgeSearch       = "knowledge_search"
)

type Request struct {
	Tool      string               `json:"tool"`
	Binding   types.BindingContext `json:"binding"`
	Arguments map[string]any       `json:"arguments"`
}
type Response struct {
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

func ValidateRequest(r Request) error {
	if r.Tool == "" {
		return errors.New("cognition tool is required")
	}
	if r.Binding.TenantID == 0 || r.Binding.BindingID == "" {
		return errors.New("valid binding context is required")
	}
	if r.Binding.ExpiresAt.IsZero() {
		return errors.New("binding context expiry is required")
	}
	return nil
}
