package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type wikiUpdateIssueTool struct {
	BaseTool
	wikiService interfaces.WikiPageService
	kbIDs       []string
}

func NewWikiUpdateIssueTool(wikiService interfaces.WikiPageService, kbIDs []string) types.Tool {
	return &wikiUpdateIssueTool{
		BaseTool: NewBaseTool(
			ToolWikiUpdateIssue,
			"Update the status of a specific wiki page issue (e.g., set it to 'resolved' or 'ignored').",
			json.RawMessage(`{
  "type": "object",
  "properties": {
    "issue_id": {
      "type": "string",
      "description": "The ID of the issue to update."
    },
    "knowledge_base_id": {
      "type": "string",
      "description": "The knowledge base that owns the issue."
    },
    "status": {
      "type": "string",
      "enum": ["resolved", "ignored", "pending"],
      "description": "The new status for the issue."
    }
  },
  "required": ["knowledge_base_id", "issue_id", "status"]
}`),
		),
		wikiService: wikiService,
		kbIDs:       kbIDs,
	}
}

func (t *wikiUpdateIssueTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var params struct {
		KnowledgeBaseID string `json:"knowledge_base_id"`
		IssueID         string `json:"issue_id"`
		Status          string `json:"status"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return &types.ToolResult{Success: false, Error: "Invalid parameters: " + err.Error()}, nil
	}

	if params.KnowledgeBaseID == "" {
		return &types.ToolResult{Success: false, Error: "knowledge_base_id is required"}, nil
	}
	if params.IssueID == "" {
		return &types.ToolResult{Success: false, Error: "issue_id is required"}, nil
	}
	if params.Status == "" {
		return &types.ToolResult{Success: false, Error: "status is required"}, nil
	}

	if len(t.kbIDs) == 0 {
		return &types.ToolResult{Success: false, Error: "No knowledge bases available"}, nil
	}
	allowed := false
	for _, kbID := range t.kbIDs {
		if kbID == params.KnowledgeBaseID {
			allowed = true
			break
		}
	}
	if !allowed {
		return &types.ToolResult{Success: false, Error: "knowledge_base_id is outside the configured tool scope"}, nil
	}

	// Update issue status
	err := t.wikiService.UpdateIssueStatus(ctx, params.KnowledgeBaseID, params.IssueID, params.Status)
	if err != nil {
		return &types.ToolResult{Success: false, Error: "Failed to update issue status: " + err.Error()}, nil
	}

	return &types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully updated issue %s to status '%s'", params.IssueID, params.Status),
	}, nil
}
