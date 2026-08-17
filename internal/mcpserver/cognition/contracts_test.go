package cognition

import (
	"testing"
)

func TestValidateRequestRejectsUnknownTool(t *testing.T) {
	if err := ValidateRequest(Request{Tool: "memory_submit_candidate"}); err == nil {
		t.Fatal("expected unknown tool validation error")
	}
}

func TestValidateRequestAcceptsFrozenToolSet(t *testing.T) {
	for _, tool := range []string{
		ToolMemoryGetContext, ToolMemorySearch, ToolMemoryCaptureTurn,
		ToolMemoryConfirmCandidate, ToolKnowledgeSearch, ToolWikiGetPage,
		ToolDocumentRead, ToolContextAssemble,
	} {
		if err := ValidateRequest(Request{Tool: tool}); err != nil {
			t.Fatalf("tool %s: %v", tool, err)
		}
	}
}
