package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

func TestWikiToolsRejectExplicitKnowledgeBaseOutsideServerScope(t *testing.T) {
	tests := []struct {
		name string
		tool interface {
			Execute(context.Context, json.RawMessage) (*types.ToolResult, error)
		}
		args json.RawMessage
	}{
		{
			name: "read",
			tool: NewWikiReadPageTool(nil, nil, []WikiScope{{KnowledgeBaseID: "kb-allowed"}}),
			args: json.RawMessage(`{"slug":"memory/page","knowledge_base_id":"kb-forbidden"}`),
		},
		{
			name: "search",
			tool: NewWikiSearchTool(nil, nil, []WikiScope{{KnowledgeBaseID: "kb-allowed"}}),
			args: json.RawMessage(`{"queries":["secret"],"knowledge_base_id":"kb-forbidden"}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.tool.Execute(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("Execute error=%v", err)
			}
			if result == nil || result.Success || !strings.Contains(strings.ToLower(result.Error), "outside") {
				t.Fatalf("result=%+v, want fail-closed outside-scope error", result)
			}
		})
	}
}
